// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package rally

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/packages/installer"

	"github.com/magefile/mage/sh"

	"github.com/elastic/elastic-package/internal/corpusgenerator"
	"github.com/elastic/elastic-package/internal/stack"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/signal"
)

const (
	// RallyCorpusAgentDir is folder path where rally corporsa files produced by the service
	// are stored on the Rally container's filesystem.
	RallyCorpusAgentDir = "/tmp/rally_corpus"

	// BenchType defining rally benchmark
	BenchType benchrunner.Type = "rally"
)

var ErrDryRun = errors.New("dry run: rally benchmark not executed")

type rallyStat struct {
	Metric string
	Task   string
	Value  any
	Unit   string
}

type runner struct {
	options  Options
	scenario *scenario

	ctxt              servicedeployer.ServiceContext
	runtimeDataStream string
	pipelinePrefix    string
	generator         genlib.Generator
	mcollector        *collector

	corpusFile string
	trackFile  string
	reportFile string

	// Execution order of following handlers is defined in runner.TearDown() method.
	persistRallyTrackHandler func() error
	removePackageHandler     func() error
	wipeDataStreamHandler    func() error
	clearCorporaHandler      func() error
}

func NewRallyBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

func (r *runner) SetUp() error {
	return r.setUp()
}

// Run runs the system benchmarks defined under the given folder
func (r *runner) Run() (reporters.Reportable, error) {
	return r.run()
}

func (r *runner) TearDown() error {
	if r.options.DeferCleanup > 0 {
		logger.Debugf("waiting for %s before tearing down...", r.options.DeferCleanup)
		signal.Sleep(r.options.DeferCleanup)
	}

	var merr multierror.Error

	if r.persistRallyTrackHandler != nil {
		if err := r.persistRallyTrackHandler(); err != nil {
			merr = append(merr, err)
		}
		r.persistRallyTrackHandler = nil
	}

	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(); err != nil {
			merr = append(merr, err)
		}
		r.removePackageHandler = nil
	}

	if r.wipeDataStreamHandler != nil {
		if err := r.wipeDataStreamHandler(); err != nil {
			merr = append(merr, err)
		}
		r.wipeDataStreamHandler = nil
	}

	if r.clearCorporaHandler != nil {
		if err := r.clearCorporaHandler(); err != nil {
			merr = append(merr, err)
		}
		r.clearCorporaHandler = nil
	}

	if len(merr) == 0 {
		return nil
	}
	return merr
}

func (r *runner) createRallyTrackDir(locationManager *locations.LocationManager) error {
	outputDir := filepath.Join(locationManager.RallyCorpusDir(), r.ctxt.Test.RunID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

func (r *runner) setUp() error {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("reading service logs directory failed: %w", err)
	}

	rallyCorpusDir := locationManager.RallyCorpusDir()
	r.ctxt.Logs.Folder.Local = rallyCorpusDir
	r.ctxt.Logs.Folder.Agent = RallyCorpusAgentDir
	r.ctxt.Test.RunID = createRunID()

	outputDir, err := servicedeployer.CreateOutputDir(locationManager, r.ctxt.Test.RunID)
	if err != nil {
		return fmt.Errorf("could not create output dir for terraform deployer %w", err)
	}
	r.ctxt.OutputDir = outputDir

	err = r.createRallyTrackDir(locationManager)
	if err != nil {
		return fmt.Errorf("could not create local rally track dir %w", err)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
	}

	scenario, err := readConfig(r.options.PackageRootPath, r.options.BenchName, pkgManifest.Name, pkgManifest.Version)
	if err != nil {
		return err
	}
	r.scenario = scenario

	err = r.installPackage()
	if err != nil {
		return fmt.Errorf("error installing package: %w", err)
	}

	if r.scenario.Corpora.Generator != nil {
		var err error
		r.generator, err = r.initializeGenerator()
		if err != nil {
			return fmt.Errorf("can't initialize generator: %w", err)
		}
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(
		filepath.Join(
			getDataStreamPath(r.options.PackageRootPath, r.scenario.DataStream.Name),
			packages.DataStreamManifestFile,
		),
	)
	if err != nil {
		return fmt.Errorf("reading data stream manifest failed: %w", err)
	}

	r.runtimeDataStream = fmt.Sprintf(
		"%s-%s.%s-ep",
		dataStreamManifest.Type,
		pkgManifest.Name,
		dataStreamManifest.Name,
	)
	r.pipelinePrefix = fmt.Sprintf(
		"%s-%s.%s-%s",
		dataStreamManifest.Type,
		pkgManifest.Name,
		dataStreamManifest.Name,
		r.scenario.Version,
	)

	if err := r.wipeDataStreamOnSetup(); err != nil {
		return fmt.Errorf("error deleting old data in data stream: %s: %w", r.runtimeDataStream, err)
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel clearing data")
		}

		hits, err := getTotalHits(r.options.ESAPI, r.runtimeDataStream)
		return hits == 0, err
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return err
	}

	return nil
}

func (r *runner) wipeDataStreamOnSetup() error {
	// Delete old data
	logger.Debug("deleting old data in data stream...")
	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := r.deleteDataStreamDocs(r.runtimeDataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	return r.deleteDataStreamDocs(r.runtimeDataStream)
}

func (r *runner) run() (report reporters.Reportable, err error) {
	r.startMetricsColletion()
	defer r.mcollector.stop()

	// if there is a generator config, generate the data
	if r.generator != nil {
		logger.Debugf("generating corpus data to %s...", r.ctxt.Logs.Folder.Local)
		if err := r.runGenerator(r.ctxt.Logs.Folder.Local); err != nil {
			return nil, fmt.Errorf("can't generate benchmarks data corpus for data stream: %w", err)
		}
	}

	if r.options.DryRun {
		dummy := reporters.NewReport(r.scenario.Package, nil)
		return dummy, ErrDryRun
	}

	rallyStats, err := r.runRally()
	if err != nil {
		return nil, err
	}

	msum, err := r.collectAndSummarizeMetrics()
	if err != nil {
		return nil, fmt.Errorf("can't summarize metrics: %w", err)
	}

	if err := r.reindexData(); err != nil {
		return nil, fmt.Errorf("can't reindex data: %w", err)
	}

	return createReport(r.options.BenchName, r.corpusFile, r.scenario, msum, rallyStats)
}

func (r *runner) installPackage() error {
	logger.Debug("Installing package...")
	installer, err := installer.NewForPackage(installer.Options{
		Kibana:         r.options.KibanaClient,
		RootPath:       r.options.PackageRootPath,
		SkipValidation: true,
	})

	if err != nil {
		return fmt.Errorf("failed to initialize package installer: %w", err)
	}

	_, err = installer.Install()
	if err != nil {
		return fmt.Errorf("failed to install package: %w", err)
	}

	r.removePackageHandler = func() error {
		if err := installer.Uninstall(); err != nil {
			return fmt.Errorf("error removing benchmark package: %w", err)
		}

		return nil
	}

	return nil
}

func (r *runner) startMetricsColletion() {
	// TODO collect agent hosts metrics using system integration
	r.mcollector = newCollector(
		r.ctxt,
		r.options.BenchName,
		*r.scenario,
		r.options.ESAPI,
		r.options.ESMetricsAPI,
		r.options.MetricsInterval,
		r.runtimeDataStream,
		r.pipelinePrefix,
	)
	r.mcollector.start()
}

func (r *runner) collectAndSummarizeMetrics() (*metricsSummary, error) {
	r.mcollector.stop()
	sum, err := r.mcollector.summarize()
	return sum, err
}

func (r *runner) deleteDataStreamDocs(dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	resp, err := r.options.ESAPI.DeleteByQuery([]string{dataStream}, body)
	if err != nil {
		return fmt.Errorf("failed to delete data stream docs for data stream %s: %w", dataStream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Unavailable index is ok, this means that data is already not there.
		return nil
	}
	if resp.IsError() {
		return fmt.Errorf("failed to delete data stream docs for data stream %s: %s", dataStream, resp.String())
	}

	return nil
}

func (r *runner) initializeGenerator() (genlib.Generator, error) {
	totEvents := r.scenario.Corpora.Generator.TotalEvents

	config, err := r.getGeneratorConfig()
	if err != nil {
		return nil, err
	}

	fields, err := r.getGeneratorFields()
	if err != nil {
		return nil, err
	}

	tpl, err := r.getGeneratorTemplate()
	if err != nil {
		return nil, err
	}

	genlib.InitGeneratorTimeNow(time.Now())
	genlib.InitGeneratorRandSeed(time.Now().UnixNano())

	var generator genlib.Generator
	switch r.scenario.Corpora.Generator.Template.Type {
	default:
		logger.Debugf("unknown generator template type %q, defaulting to \"placeholder\"", r.scenario.Corpora.Generator.Template.Type)
		fallthrough
	case "", "placeholder":
		generator, err = genlib.NewGeneratorWithCustomTemplate(tpl, *config, fields, totEvents)
	case "gotext":
		generator, err = genlib.NewGeneratorWithTextTemplate(tpl, *config, fields, totEvents)
	}

	if err != nil {
		return nil, err
	}

	return generator, nil
}

func (r *runner) getGeneratorConfig() (*config.Config, error) {
	var (
		data []byte
		err  error
	)

	if r.scenario.Corpora.Generator.Config.Path != "" {
		configPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Config.Path))
		configPath = os.ExpandEnv(configPath)
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("can't find config file %s: %w", configPath, err)
		}
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("can't read config file %s: %w", configPath, err)
		}
	} else if len(r.scenario.Corpora.Generator.Config.Raw) > 0 {
		data, err = yaml.Marshal(r.scenario.Corpora.Generator.Config.Raw)
		if err != nil {
			return nil, fmt.Errorf("can't parse raw generator config: %w", err)
		}
	}

	cfg, err := config.LoadConfigFromYaml(data)
	if err != nil {
		return nil, fmt.Errorf("can't get generator config: %w", err)
	}

	return &cfg, nil
}

func (r *runner) getGeneratorFields() (fields.Fields, error) {
	var (
		data []byte
		err  error
	)

	if r.scenario.Corpora.Generator.Fields.Path != "" {
		fieldsPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Fields.Path))
		fieldsPath = os.ExpandEnv(fieldsPath)
		if _, err := os.Stat(fieldsPath); err != nil {
			return nil, fmt.Errorf("can't find fields file %s: %w", fieldsPath, err)
		}

		data, err = os.ReadFile(fieldsPath)
		if err != nil {
			return nil, fmt.Errorf("can't read fields file %s: %w", fieldsPath, err)
		}
	} else if len(r.scenario.Corpora.Generator.Fields.Raw) > 0 {
		data, err = yaml.Marshal(r.scenario.Corpora.Generator.Fields.Raw)
		if err != nil {
			return nil, fmt.Errorf("can't parse raw generator fields: %w", err)
		}
	}

	fields, err := fields.LoadFieldsWithTemplateFromString(context.Background(), string(data))
	if err != nil {
		return nil, fmt.Errorf("could not load fields yaml: %w", err)
	}

	return fields, nil
}

func (r *runner) getGeneratorTemplate() ([]byte, error) {
	var (
		data []byte
		err  error
	)

	if r.scenario.Corpora.Generator.Template.Path != "" {
		tplPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Template.Path))
		tplPath = os.ExpandEnv(tplPath)
		if _, err := os.Stat(tplPath); err != nil {
			return nil, fmt.Errorf("can't find template file %s: %w", tplPath, err)
		}

		data, err = os.ReadFile(tplPath)
		if err != nil {
			return nil, fmt.Errorf("can't read template file %s: %w", tplPath, err)
		}
	} else if len(r.scenario.Corpora.Generator.Template.Raw) > 0 {
		data = []byte(r.scenario.Corpora.Generator.Template.Raw)
	}

	return data, nil
}

func (r *runner) runGenerator(destDir string) error {
	corpusFile, err := os.CreateTemp(destDir, "corpus-*")
	if err != nil {
		return fmt.Errorf("cannot not create rally corpus file: %w", err)
	}
	defer corpusFile.Close()

	if err := corpusFile.Chmod(os.ModePerm); err != nil {
		return fmt.Errorf("cannot not set permission to rally corpus file: %w", err)
	}

	buf := bytes.NewBufferString("")
	var corpusDocsCount uint64
	for {
		err := r.generator.Emit(buf)
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("error while generating content for the rally corpus file: %w", err)
		}

		// TODO: this should be taken care of by the corpus generator tool, once it will be done let's remove this
		event := strings.Replace(buf.String(), "\n", "", -1)
		if _, err = corpusFile.Write([]byte(event)); err != nil {
			return fmt.Errorf("error while saving content to the rally corpus file: %w", err)
		}

		if _, err = corpusFile.Write([]byte("\n")); err != nil {
			return fmt.Errorf("error while saving newline to the rally corpus file: %w", err)
		}

		buf.Reset()
		corpusDocsCount += 1
	}

	r.corpusFile = corpusFile.Name()

	trackFile, err := os.CreateTemp(destDir, "track-*.json")
	if err != nil {
		return fmt.Errorf("cannot not create rally track file: %w", err)
	}
	r.trackFile = trackFile.Name()
	rallyTrackContent, err := corpusgenerator.GenerateRallyTrack(r.runtimeDataStream, corpusFile, corpusDocsCount)
	if err != nil {
		return fmt.Errorf("cannot not generate rally track content: %w", err)
	}
	err = os.WriteFile(r.trackFile, rallyTrackContent, os.ModePerm)
	if err != nil {
		return fmt.Errorf("cannot not save rally track content to file: %w", err)
	}
	defer trackFile.Close()

	reportFile, err := os.CreateTemp(destDir, "report-*.csv")
	if err != nil {
		return fmt.Errorf("cannot not save rally report file: %w", err)
	}
	defer reportFile.Close()

	r.reportFile = reportFile.Name()

	if r.options.RallyTrackOutputDir != "" {
		r.persistRallyTrackHandler = func() error {
			err := os.MkdirAll(r.options.RallyTrackOutputDir, os.ModeDir)
			if err != nil {
				return fmt.Errorf("cannot not create rally track output dir: %w", err)
			}

			persistedRallyTrack := filepath.Join(r.options.RallyTrackOutputDir, fmt.Sprintf("track-%s.json", r.runtimeDataStream))
			err = sh.Copy(persistedRallyTrack, trackFile.Name())
			if err != nil {
				return fmt.Errorf("cannot not copy rally track to file in output dir: %w", err)
			}

			persistedCorpus := filepath.Join(r.options.RallyTrackOutputDir, filepath.Base(corpusFile.Name()))
			err = sh.Copy(persistedCorpus, corpusFile.Name())
			if err != nil {
				err = fmt.Errorf("cannot not copy rally corpus to file in output dir: %w", err)
				return errors.Join(os.Remove(persistedRallyTrack), err)
			}

			logger.Infof("rally track and corpus saved at: %s", r.options.RallyTrackOutputDir)
			return nil
		}
	}

	r.clearCorporaHandler = func() error {
		return errors.Join(
			os.Remove(r.corpusFile),
			os.Remove(r.reportFile),
			os.Remove(r.trackFile),
		)
	}

	return r.generator.Close()
}

func (r *runner) runRally() ([]rallyStat, error) {
	logger.Debug("running rally...")
	profileConfig, err := stack.StackInitConfig(r.options.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}

	elasticsearchHost, found := os.LookupEnv(stack.ElasticsearchHostEnv)
	if !found {
		status, err := stack.Status(stack.Options{Profile: r.options.Profile})
		if err != nil {
			return nil, fmt.Errorf("failed to check status of stack in current profile: %w", err)
		}
		if len(status) == 0 {
			return nil, stack.ErrUnavailableStack
		}

		elasticsearchHost = profileConfig.ElasticsearchHostPort
		logger.Debugf("Configuring rally with Elasticsearch host from current profile (profile: %s, host: %q)", r.options.Profile.ProfileName, elasticsearchHost)
	}

	elasticsearchPassword, found := os.LookupEnv(stack.ElasticsearchPasswordEnv)
	if !found {
		elasticsearchPassword = profileConfig.ElasticsearchPassword
	}
	elasticsearchUsername, found := os.LookupEnv(stack.ElasticsearchUsernameEnv)
	if !found {
		elasticsearchUsername = profileConfig.ElasticsearchUsername
	}

	_, err = exec.LookPath("esrally")
	if err != nil {
		return nil, errors.New("could not run esrally track in path: esrally not found, please follow instruction at https://esrally.readthedocs.io/en/stable/install.html")
	}

	cmd := exec.Command(
		"esrally",
		"race",
		"--race-id="+r.ctxt.Test.RunID,
		"--report-format=csv",
		fmt.Sprintf(`--report-file=%s`, r.reportFile),
		fmt.Sprintf(`--target-hosts={"default":["%s"]}`, elasticsearchHost),
		fmt.Sprintf(`--track-path=%s`, r.trackFile),
		fmt.Sprintf(`--client-options={"default":{"basic_auth_user":"%s","basic_auth_password":"%s","use_ssl":true,"verify_certs":false}}`, elasticsearchUsername, elasticsearchPassword),
		"--pipeline=benchmark-only",
	)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not run esrally track in path: %s (stdout=%s): %w", r.ctxt.Logs.Folder.Local, output, err)
	}

	reportCSV, err := os.Open(r.reportFile)
	if err != nil {
		return nil, fmt.Errorf("could not open esrally report in path: %s (stderr=%q): %w", r.ctxt.Logs.Folder.Local, errOutput.String(), err)
	}

	reader := csv.NewReader(reportCSV)

	stats := make([]rallyStat, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not read esrally report in path: %s (stderr=%q): %w", r.ctxt.Logs.Folder.Local, errOutput.String(), err)
		}

		stats = append(stats, rallyStat{Metric: record[0], Task: record[1], Value: record[2], Unit: record[3]})
	}

	return stats, nil
}

// reindexData will read all data generated during the benchmark and will reindex it to the metrisctore
func (r *runner) reindexData() error {
	if !r.options.ReindexData {
		return nil
	}
	if r.options.ESMetricsAPI == nil {
		return errors.New("the option to reindex data is set, but the metricstore was not initialized")
	}

	logger.Debug("starting reindexing of data...")

	logger.Debug("getting orignal mappings...")
	// Get the mapping from the source data stream
	mappingRes, err := r.options.ESAPI.Indices.GetMapping(
		r.options.ESAPI.Indices.GetMapping.WithIndex(r.runtimeDataStream),
	)
	if err != nil {
		return fmt.Errorf("error getting mapping: %w", err)
	}
	defer mappingRes.Body.Close()
	if mappingRes.IsError() {
		return fmt.Errorf("error getting mapping: %s", mappingRes)
	}

	body, err := io.ReadAll(mappingRes.Body)
	if err != nil {
		return fmt.Errorf("error reading mapping body: %w", err)
	}

	mappings := map[string]struct {
		Mappings json.RawMessage
	}{}

	if err := json.Unmarshal(body, &mappings); err != nil {
		return fmt.Errorf("error unmarshaling mappings: %w", err)
	}

	if len(mappings) != 1 {
		return fmt.Errorf("exactly 1 mapping was expected, got %d", len(mappings))
	}

	var mapping string
	for _, v := range mappings {
		mapping = string(v.Mappings)
	}

	reader := bytes.NewReader(
		[]byte(fmt.Sprintf(`{
			"settings": {"number_of_replicas":0},
			"mappings": %s
		}`, mapping)),
	)

	indexName := fmt.Sprintf("bench-reindex-%s-%s", r.runtimeDataStream, r.ctxt.Test.RunID)

	logger.Debugf("creating %s index in metricstore...", indexName)

	createRes, err := r.options.ESMetricsAPI.Indices.Create(
		indexName,
		r.options.ESMetricsAPI.Indices.Create.WithBody(reader),
	)
	if err != nil {
		return fmt.Errorf("could not create index: %w", err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return fmt.Errorf("got a response error while creating index: %s", createRes)
	}

	bodyReader := strings.NewReader(`{"query":{"match_all":{}}}`)

	logger.Debug("starting scrolling of events...")
	res, err := r.options.ESAPI.Search(
		r.options.ESAPI.Search.WithIndex(r.runtimeDataStream),
		r.options.ESAPI.Search.WithBody(bodyReader),
		r.options.ESAPI.Search.WithScroll(time.Minute),
		r.options.ESAPI.Search.WithSize(10000),
	)
	if err != nil {
		return fmt.Errorf("error executing search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("error executing search: %s", res)
	}

	// Iterate through the search results using the Scroll API
	for {
		var sr searchResponse
		if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
			return fmt.Errorf("error decoding search response: %w", err)
		}

		if sr.Error != nil {
			return fmt.Errorf("error searching for documents: %s", sr.Error.Reason)
		}

		if len(sr.Hits) == 0 {
			break
		}

		err := r.bulkMetrics(indexName, sr)
		if err != nil {
			return err
		}
	}

	logger.Debug("reindexing operation finished")
	return nil
}

type searchResponse struct {
	Error *struct {
		Reason string `json:"reson"`
	} `json:"error"`
	ScrollID string `json:"_scroll_id"`
	Hits     []struct {
		ID     string                 `json:"_id"`
		Source map[string]interface{} `json:"_source"`
	} `json:"hits"`
}

func (r *runner) bulkMetrics(indexName string, sr searchResponse) error {
	var bulkBodyBuilder strings.Builder
	for _, hit := range sr.Hits {
		bulkBodyBuilder.WriteString(fmt.Sprintf("{\"index\":{\"_index\":\"%s\",\"_id\":\"%s\"}}\n", indexName, hit.ID))
		enriched := r.enrichEventWithBenchmarkMetadata(hit.Source)
		src, err := json.Marshal(enriched)
		if err != nil {
			return fmt.Errorf("error decoding _source: %w", err)
		}
		bulkBodyBuilder.WriteString(fmt.Sprintf("%s\n", string(src)))
	}

	logger.Debugf("bulk request of %d events...", len(sr.Hits))

	resp, err := r.options.ESMetricsAPI.Bulk(strings.NewReader(bulkBodyBuilder.String()))
	if err != nil {
		return fmt.Errorf("error performing the bulk index request: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("error performing the bulk index request: %s", resp.String())
	}

	if sr.ScrollID == "" {
		return errors.New("error getting scroll ID")
	}

	resp, err = r.options.ESAPI.Scroll(
		r.options.ESAPI.Scroll.WithScrollID(sr.ScrollID),
		r.options.ESAPI.Scroll.WithScroll(time.Minute),
	)
	if err != nil {
		return fmt.Errorf("error executing scroll: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("error executing scroll: %s", resp.String())
	}

	return nil
}

type benchMeta struct {
	Info struct {
		Benchmark string `json:"benchmark"`
		RunID     string `json:"run_id"`
	} `json:"info"`
	Parameters scenario `json:"parameter"`
}

func (r *runner) enrichEventWithBenchmarkMetadata(e map[string]interface{}) map[string]interface{} {
	var m benchMeta
	m.Info.Benchmark = r.options.BenchName
	m.Info.RunID = r.ctxt.Test.RunID
	m.Parameters = *r.scenario
	e["benchmark_metadata"] = m
	return e
}

func getTotalHits(esapi *elasticsearch.API, dataStream string) (int, error) {
	resp, err := esapi.Count(
		esapi.Count.WithIndex(dataStream),
		esapi.Count.WithIgnoreUnavailable(true),
	)
	if err != nil {
		return 0, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return 0, fmt.Errorf("failed to get hits count: %s", resp.String())
	}

	var results struct {
		Count int
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, fmt.Errorf("could not decode search results response: %w", err)
	}

	numHits := results.Count
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	return numHits, nil
}

func waitUntilTrue(fn func() (bool, error), timeout time.Duration) (bool, error) {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	retryTicker := time.NewTicker(5 * time.Second)
	defer retryTicker.Stop()

	for {
		result, err := fn()
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}

		select {
		case <-retryTicker.C:
			continue
		case <-timeoutTimer.C:
			return false, nil
		}
	}
}

func createRunID() string {
	return uuid.New().String()
}

func getDataStreamPath(packageRoot, dataStream string) string {
	return filepath.Join(packageRoot, "data_stream", dataStream)
}
