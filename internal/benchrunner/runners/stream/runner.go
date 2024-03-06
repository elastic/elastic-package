// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/elastic/elastic-package/internal/packages/installer"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/signal"
)

type runner struct {
	options   Options
	scenarios map[string]*scenario

	ctxt               servicedeployer.ServiceContext
	runtimeDataStreams map[string]string
	generators         map[string]genlib.Generator
	backFillGenerators map[string]genlib.Generator
	errChanGenerators  chan error

	wg   sync.WaitGroup
	done chan struct{}

	// Execution order of following handlers is defined in runner.TearDown() method.
	removePackageHandler  func() error
	wipeDataStreamHandler func() error
}

func NewStreamBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

func (r *runner) SetUp() error {
	return r.setUp()
}

// Run runs the system benchmarks defined under the given folder
func (r *runner) Run() (reporters.Reportable, error) {
	return nil, r.run()
}

func (r *runner) TearDown() error {
	r.wg.Wait()

	if !r.options.PerformCleanup {
		r.removePackageHandler = nil
		r.wipeDataStreamHandler = nil
		return nil
	}

	var merr multierror.Error

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

	if len(merr) == 0 {
		return nil
	}
	return merr
}

func (r *runner) setUp() error {
	r.generators = make(map[string]genlib.Generator)
	r.backFillGenerators = make(map[string]genlib.Generator)
	r.errChanGenerators = make(chan error)
	r.done = make(chan struct{})

	r.runtimeDataStreams = make(map[string]string)

	r.ctxt.Test.RunID = common.NewRunID()

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
	}

	scenarios, err := readScenarios(r.options.PackageRootPath, r.options.BenchName, pkgManifest.Name, pkgManifest.Version)
	if err != nil {
		return err
	}
	r.scenarios = scenarios

	if err = r.installPackage(); err != nil {
		return fmt.Errorf("error installing package: %w", err)
	}

	err = r.collectGenerators()
	if err != nil {
		return fmt.Errorf("can't initialize generator: %w", err)
	}

	for scenarioName, scenario := range r.scenarios {
		var err error
		dataStreamManifest, err := packages.ReadDataStreamManifest(
			filepath.Join(
				common.DataStreamPath(r.options.PackageRootPath, scenario.DataStream.Name),
				packages.DataStreamManifestFile,
			),
		)
		if err != nil {
			return fmt.Errorf("reading data stream manifest failed: %w", err)
		}
		r.runtimeDataStreams[scenarioName] = fmt.Sprintf(
			"%s-%s.%s-ep",
			dataStreamManifest.Type,
			pkgManifest.Name,
			dataStreamManifest.Name,
		)
	}

	if !r.options.PerformCleanup {
		return nil
	}

	if err := r.wipeDataStreamsOnSetup(); err != nil {
		return fmt.Errorf("error cleaning up old data in data streams: %w", err)
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel clearing data")
		}

		totalHits := 0
		for _, runtimeDataStream := range r.runtimeDataStreams {
			hits, err := getTotalHits(r.options.ESAPI, runtimeDataStream)
			if err != nil {
				return false, err
			}
			totalHits += hits
		}
		return totalHits == 0, nil
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return err
	}

	return nil
}

func (r *runner) wipeDataStreamsOnSetup() error {
	// Delete old data
	logger.Debug("deleting old data in data stream...")
	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		for _, runtimeDataStream := range r.runtimeDataStreams {
			if err := r.deleteDataStreamDocs(runtimeDataStream); err != nil {
				return fmt.Errorf("error deleting data in data stream: %w", err)
			}
		}
		return nil
	}

	for _, runtimeDataStream := range r.runtimeDataStreams {
		if err := r.deleteDataStreamDocs(runtimeDataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
	}

	return nil
}

func (r *runner) run() (err error) {
	r.streamData()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err = <-r.errChanGenerators:
			close(r.done)
			return err
		case <-ticker.C:
			if signal.SIGINT() {
				close(r.done)
				return nil
			}
		}
	}
}

func (r *runner) installPackage() error {
	return r.installPackageFromPackageRoot()
}

func (r *runner) installPackageFromPackageRoot() error {
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

func (r *runner) initializeGenerator(tpl []byte, config genlib.Config, fields genlib.Fields, scenario *scenario, backFill time.Duration, totEvents uint64) (genlib.Generator, error) {
	timestampConfig := genlib.ConfigField{Name: r.options.TimestampField}
	if backFill < 0 {
		timestampConfig.Period = backFill
	}

	config.SetField(r.options.TimestampField, timestampConfig)

	switch scenario.Corpora.Generator.Template.Type {
	default:
		logger.Debugf("unknown generator template type %q, defaulting to \"placeholder\"", scenario.Corpora.Generator.Template.Type)
		fallthrough
	case "", "placeholder":
		return genlib.NewGeneratorWithCustomTemplate(tpl, config, fields, totEvents)
	case "gotext":
		return genlib.NewGeneratorWithTextTemplate(tpl, config, fields, totEvents)
	}
}
func (r *runner) collectGenerators() error {
	for scenarioName, scenario := range r.scenarios {
		config, err := r.getGeneratorConfig(scenario)
		if err != nil {
			return err
		}

		fields, err := r.getGeneratorFields(scenario)
		if err != nil {
			return err
		}

		tpl, err := r.getGeneratorTemplate(scenario)
		if err != nil {
			return err
		}

		genlib.InitGeneratorTimeNow(time.Now())
		genlib.InitGeneratorRandSeed(time.Now().UnixNano())

		generator, err := r.initializeGenerator(tpl, *config, fields, scenario, 0, 0)
		if err != nil {
			return err
		}

		r.generators[scenarioName] = generator

		if r.options.BackFill >= 0 {
			continue
		}

		// backfill is a negative duration, make it positive, find how many periods in the backfill and multiply by events for periodk
		totEvents := uint64((-1*r.options.BackFill)/r.options.PeriodDuration) * r.options.EventsPerPeriod

		generator, err = r.initializeGenerator(tpl, *config, fields, scenario, r.options.BackFill, totEvents)
		if err != nil {
			return err
		}

		r.backFillGenerators[scenarioName] = generator
	}

	return nil
}

func (r *runner) getGeneratorConfig(scenario *scenario) (*config.Config, error) {
	var (
		data []byte
		err  error
	)

	if scenario.Corpora.Generator.Config.Path != "" {
		configPath := filepath.Clean(filepath.Join(devPath, scenario.Corpora.Generator.Config.Path))
		configPath = os.ExpandEnv(configPath)
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("can't find config file %s: %w", configPath, err)
		}
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("can't read config file %s: %w", configPath, err)
		}
	} else if len(scenario.Corpora.Generator.Config.Raw) > 0 {
		data, err = yaml.Marshal(scenario.Corpora.Generator.Config.Raw)
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

func (r *runner) getGeneratorFields(scenario *scenario) (fields.Fields, error) {
	var (
		data []byte
		err  error
	)

	if scenario.Corpora.Generator.Fields.Path != "" {
		fieldsPath := filepath.Clean(filepath.Join(devPath, scenario.Corpora.Generator.Fields.Path))
		fieldsPath = os.ExpandEnv(fieldsPath)
		if _, err := os.Stat(fieldsPath); err != nil {
			return nil, fmt.Errorf("can't find fields file %s: %w", fieldsPath, err)
		}

		data, err = os.ReadFile(fieldsPath)
		if err != nil {
			return nil, fmt.Errorf("can't read fields file %s: %w", fieldsPath, err)
		}
	} else if len(scenario.Corpora.Generator.Fields.Raw) > 0 {
		data, err = yaml.Marshal(scenario.Corpora.Generator.Fields.Raw)
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

func (r *runner) getGeneratorTemplate(scenario *scenario) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	if scenario.Corpora.Generator.Template.Path != "" {
		tplPath := filepath.Clean(filepath.Join(devPath, scenario.Corpora.Generator.Template.Path))
		tplPath = os.ExpandEnv(tplPath)
		if _, err := os.Stat(tplPath); err != nil {
			return nil, fmt.Errorf("can't find template file %s: %w", tplPath, err)
		}

		data, err = os.ReadFile(tplPath)
		if err != nil {
			return nil, fmt.Errorf("can't read template file %s: %w", tplPath, err)
		}
	} else if len(scenario.Corpora.Generator.Template.Raw) > 0 {
		data = []byte(scenario.Corpora.Generator.Template.Raw)
	}

	return data, nil
}

func (r *runner) collectBulkRequestBody(indexName, scenarioName string, buf *bytes.Buffer, generator genlib.Generator, bulkBodyBuilder strings.Builder) (strings.Builder, error) {
	err := generator.Emit(buf)
	if err != nil {
		return bulkBodyBuilder, err
	}

	var event map[string]any
	err = json.Unmarshal(buf.Bytes(), &event)
	if err != nil {
		logger.Debugf("Problem found when unmarshalling document: %s", buf.String())
		return bulkBodyBuilder, fmt.Errorf("failed to unmarshal json event, check your benchmark template for scenario %s: %w", scenarioName, err)
	}
	enriched := r.enrichEventWithBenchmarkMetadata(event)
	src, err := json.Marshal(enriched)
	if err != nil {
		return bulkBodyBuilder, err
	}

	bulkBodyBuilder.WriteString(fmt.Sprintf("{\"create\":{\"_index\":\"%s\"}}\n", indexName))
	bulkBodyBuilder.WriteString(fmt.Sprintf("%s\n", string(src)))

	buf.Reset()

	return bulkBodyBuilder, nil
}

func (r *runner) performBulkRequest(bulkRequest string) error {
	resp, err := r.options.ESAPI.Bulk(strings.NewReader(bulkRequest))
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type bodyErrors struct {
		Errors bool  `json:"errors"`
		Items  []any `json:"items"`
	}

	var errors bodyErrors
	err = json.Unmarshal(body, &errors)
	if err != nil {
		return err
	}

	if errors.Errors {
		logger.Debug("Error in Elasticsearch bulk request: %s", string(body))
		return fmt.Errorf("%d failed", len(errors.Items))
	}

	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("%s", resp.String())
	}

	return nil
}

func (r *runner) streamData() {
	logger.Debug("streaming data...")
	r.wg.Add(len(r.backFillGenerators) + len(r.generators))
	for scenarioName, generator := range r.generators {
		go func(scenarioName string, generator genlib.Generator) {
			defer r.wg.Done()
			ticker := time.NewTicker(r.options.PeriodDuration)
			indexName := r.runtimeDataStreams[scenarioName]
			for {
				select {
				case <-r.done:
					return
				case <-ticker.C:
					logger.Debugf("bulk request of %d events on %s...", r.options.EventsPerPeriod, indexName)
					var bulkBodyBuilder strings.Builder
					buf := bytes.NewBufferString("")
					for i := uint64(0); i < r.options.EventsPerPeriod; i++ {
						var err error
						bulkBodyBuilder, err = r.collectBulkRequestBody(indexName, scenarioName, buf, generator, bulkBodyBuilder)
						if err == io.EOF {
							break
						}

						if err != nil {
							r.errChanGenerators <- fmt.Errorf("error while generating event for streaming: %w", err)
							return
						}
					}

					err := r.performBulkRequest(bulkBodyBuilder.String())
					if err != nil {
						r.errChanGenerators <- fmt.Errorf("error performing bulk request: %w", err)
						return
					}
				}
			}
		}(scenarioName, generator)
	}

	for scenarioName, backFillGenerator := range r.backFillGenerators {
		go func(scenarioName string, generator genlib.Generator) {
			defer r.wg.Done()
			var bulkBodyBuilder strings.Builder
			indexName := r.runtimeDataStreams[scenarioName]
			logger.Debugf("bulk request of %s backfill events on %s...", r.options.BackFill.String(), indexName)
			buf := bytes.NewBufferString("")
			for {
				var err error
				bulkBodyBuilder, err = r.collectBulkRequestBody(indexName, scenarioName, buf, generator, bulkBodyBuilder)
				if err == io.EOF {
					break
				}

				if err != nil {
					r.errChanGenerators <- fmt.Errorf("error while generating event for streaming: %w", err)
					return
				}
			}

			err := r.performBulkRequest(bulkBodyBuilder.String())
			if err != nil {
				r.errChanGenerators <- fmt.Errorf("error performing bulk request: %w", err)
				return
			}
		}(scenarioName, backFillGenerator)
	}
}

type benchMeta struct {
	Info struct {
		Benchmark string `json:"benchmark"`
		RunID     string `json:"run_id"`
	} `json:"info"`
}

func (r *runner) enrichEventWithBenchmarkMetadata(e map[string]any) map[string]interface{} {
	var m benchMeta
	m.Info.Benchmark = r.options.BenchName
	m.Info.RunID = r.ctxt.Test.RunID
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
