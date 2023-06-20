// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/system/servicedeployer"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
)

const (
	// ServiceLogsAgentDir is folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	ServiceLogsAgentDir = "/tmp/service_logs"

	waitForDataDefaultTimeout = 10 * time.Minute
)

const (
	// BenchType defining system benchmark
	BenchType benchrunner.Type = "system"
)

type runner struct {
	options  Options
	scenario *scenario

	ctxt              servicedeployer.ServiceContext
	benchPolicy       *kibana.Policy
	runtimeDataStream string
	pipelinePrefix    string
	generator         genlib.Generator
	mcollector        *collector
	corporaFile       string

	// Execution order of following handlers is defined in runner.TearDown() method.
	deletePolicyHandler     func() error
	resetAgentPolicyHandler func() error
	shutdownServiceHandler  func() error
	wipeDataStreamHandler   func() error
	clearCorporaHandler     func() error
}

func NewSystemBenchmark(opts Options) benchrunner.Runner {
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

	if r.resetAgentPolicyHandler != nil {
		if err := r.resetAgentPolicyHandler(); err != nil {
			merr = append(merr, err)
		}
		r.resetAgentPolicyHandler = nil
	}

	if r.deletePolicyHandler != nil {
		if err := r.deletePolicyHandler(); err != nil {
			merr = append(merr, err)
		}
		r.deletePolicyHandler = nil
	}

	if r.shutdownServiceHandler != nil {
		if err := r.shutdownServiceHandler(); err != nil {
			merr = append(merr, err)
		}
		r.shutdownServiceHandler = nil
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

func (r *runner) setUp() error {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("reading service logs directory failed: %w", err)
	}

	serviceLogsDir := locationManager.ServiceLogDir()
	r.ctxt.Logs.Folder.Local = serviceLogsDir
	r.ctxt.Logs.Folder.Agent = ServiceLogsAgentDir
	r.ctxt.Bench.RunID = createRunID()

	scenario, err := readConfig(r.options.PackageRootPath, r.options.BenchName, r.ctxt)
	if err != nil {
		return err
	}
	r.scenario = scenario

	if r.scenario.Corpora.Generator != nil {
		var err error
		r.generator, err = r.initializeGenerator()
		if err != nil {
			return fmt.Errorf("can't initialize generator: %w", err)
		}
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
	}

	policy, err := r.createBenchmarkPolicy(pkgManifest)
	if err != nil {
		return err
	}
	r.benchPolicy = policy

	// Delete old data
	logger.Debug("deleting old data in data stream...")
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
		"%s-%s.%s-%s",
		dataStreamManifest.Type,
		pkgManifest.Name,
		dataStreamManifest.Name,
		policy.Namespace,
	)
	r.pipelinePrefix = fmt.Sprintf(
		"%s-%s.%s-%s",
		dataStreamManifest.Type,
		pkgManifest.Name,
		dataStreamManifest.Name,
		r.scenario.Version,
	)

	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := r.deleteDataStreamDocs(r.runtimeDataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	if err := r.deleteDataStreamDocs(r.runtimeDataStream); err != nil {
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

func (r *runner) run() (report reporters.Reportable, err error) {
	var service servicedeployer.DeployedService
	if r.scenario.Corpora.InputService != nil {
		// Setup service.
		logger.Debug("setting up service...")
		serviceDeployer, err := servicedeployer.Factory(servicedeployer.FactoryOptions{
			RootPath: r.options.PackageRootPath,
		})

		if err != nil {
			return nil, fmt.Errorf("could not create service runner: %w", err)
		}

		r.ctxt.Name = r.scenario.Corpora.InputService.Name
		service, err = serviceDeployer.SetUp(r.ctxt)
		if err != nil {
			return nil, fmt.Errorf("could not setup service: %w", err)
		}

		r.ctxt = service.Context()
		r.shutdownServiceHandler = func() error {
			logger.Debug("tearing down service...")
			if err := service.TearDown(); err != nil {
				return fmt.Errorf("error tearing down service: %w", err)
			}

			return nil
		}
	}

	r.startMetricsColletion()

	// if there is a generator config, generate the data
	if r.generator != nil {
		logger.Debugf("generating corpus data to %s...", r.ctxt.Logs.Folder.Local)
		if err := r.runGenerator(r.ctxt.Logs.Folder.Local); err != nil {
			return nil, fmt.Errorf("can't generate benchmarks data corpus for data stream: %w", err)
		}
	}

	// once data is generated, enroll agents and assign policy
	if err := r.enrollAgents(); err != nil {
		return nil, err
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if r.scenario.Corpora.InputService != nil && r.scenario.Corpora.InputService.Signal != "" {
		if err = service.Signal(r.scenario.Corpora.InputService.Signal); err != nil {
			return nil, fmt.Errorf("failed to notify benchmark service: %w", err)
		}
	}

	if err := r.waitUntilBenchmarkFinishes(); err != nil {
		return nil, err
	}

	msum, err := r.collectAndSummarizeMetrics()
	if err != nil {
		return nil, fmt.Errorf("can't summarize metrics: %w", err)
	}

	// TODO reindex if configured and es metricstore is set

	return createReport(r.options.BenchName, r.corporaFile, r.scenario, msum)
}

func (r *runner) startMetricsColletion() {
	// TODO send metrics to es metricstore if set
	// TODO collect agent hosts metrics using system integration
	r.mcollector = newCollector(
		r.ctxt,
		r.options.ESAPI,
		r.options.MetricsInterval,
		r.scenario.WarmupTimePeriod,
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
	_, err := r.options.ESAPI.DeleteByQuery([]string{dataStream}, body)
	if err != nil {
		return err
	}
	return nil
}

func (r *runner) createBenchmarkPolicy(pkgManifest *packages.PackageManifest) (*kibana.Policy, error) {
	// Configure package (single data stream) via Ingest Manager APIs.
	logger.Debug("creating benchmark policy...")
	benchTime := time.Now().Format("20060102T15:04:05Z")
	p := kibana.Policy{
		Name:              fmt.Sprintf("ep-bench-%s-%s", r.options.BenchName, benchTime),
		Description:       fmt.Sprintf("policy created by elastic-package for benchmark %s", r.options.BenchName),
		Namespace:         "ep",
		MonitoringEnabled: []string{"logs", "metrics"},
	}

	policy, err := r.options.KibanaClient.CreatePolicy(p)
	if err != nil {
		return nil, err
	}

	packagePolicy, err := r.createPackagePolicy(pkgManifest, policy)
	if err != nil {
		return nil, err
	}

	r.deletePolicyHandler = func() error {
		var merr multierror.Error

		logger.Debug("deleting benchmark package policy...")
		if err := r.options.KibanaClient.DeletePackagePolicy(*packagePolicy); err != nil {
			merr = append(merr, fmt.Errorf("error cleaning up benchmark package policy: %w", err))
		}

		logger.Debug("deleting benchmark policy...")
		if err := r.options.KibanaClient.DeletePolicy(*policy); err != nil {
			merr = append(merr, fmt.Errorf("error cleaning up benchmark policy: %w", err))
		}

		if len(merr) > 0 {
			return merr
		}

		return nil
	}

	return policy, nil
}

func (r *runner) createPackagePolicy(pkgManifest *packages.PackageManifest, p *kibana.Policy) (*kibana.PackagePolicy, error) {
	logger.Debug("creating package policy...")

	if r.scenario.Version == "" {
		r.scenario.Version = pkgManifest.Version
	}

	if r.scenario.Package == "" {
		r.scenario.Package = pkgManifest.Name
	}

	// TODO: add ability to define which policy template to use
	pp := kibana.PackagePolicy{
		Namespace: "ep",
		PolicyID:  p.ID,
		Vars:      r.scenario.Vars,
		Force:     true,
		Inputs: map[string]kibana.PackagePolicyInput{
			fmt.Sprintf("%s-%s", pkgManifest.PolicyTemplates[0].Name, r.scenario.Input): {
				Enabled: true,
				Streams: map[string]kibana.PackagePolicyStream{
					fmt.Sprintf("%s.%s", pkgManifest.Name, r.scenario.DataStream.Name): {
						Enabled: true,
						Vars:    r.scenario.DataStream.Vars,
					},
				},
			},
		},
	}
	pp.Package.Name = pkgManifest.Name
	pp.Package.Version = r.scenario.Version

	policy, err := r.options.KibanaClient.CreatePackagePolicy(pp)
	if err != nil {
		return nil, err
	}

	return policy, nil
}

func (r *runner) initializeGenerator() (genlib.Generator, error) {
	totSizeInBytes, err := humanize.ParseBytes(r.scenario.Corpora.Generator.Size)
	if err != nil {
		return nil, err
	}

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

	var generator genlib.Generator
	switch r.scenario.Corpora.Generator.Template.Type {
	default:
		logger.Debugf("unknown generator template type %q, defaulting to \"placeholder\"", r.scenario.Corpora.Generator.Template.Type)
		fallthrough
	case "", "placeholder":
		generator, err = genlib.NewGeneratorWithCustomTemplate(tpl, *config, fields, totSizeInBytes)
	case "gotext":
		generator, err = genlib.NewGeneratorWithTextTemplate(tpl, *config, fields, totSizeInBytes)
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
		data, err = yaml.Marshal(r.scenario.Corpora.Generator.Config.Raw)
		if err != nil {
			return nil, fmt.Errorf("can't parse raw generator config: %w", err)
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
	state := genlib.NewGenState()

	f, err := os.CreateTemp(destDir, "corpus-*")
	if err != nil {
		return err
	}
	defer f.Close()

	buf := bytes.NewBufferString("")
	var corpusDocsCount uint64
	for {
		err := r.generator.Emit(state, buf)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		// TODO: this should be taken care of by the corpus generator tool, once it will be done let's remove this
		replacer := strings.NewReplacer("\n", "")
		event := replacer.Replace(buf.String())
		if _, err = f.Write([]byte(event)); err != nil {
			return err
		}

		if _, err = f.Write([]byte("\n")); err != nil {
			return err
		}

		buf.Reset()
		corpusDocsCount += 1
	}

	r.corporaFile = f.Name()
	r.clearCorporaHandler = func() error {
		return os.Remove(r.corporaFile)
	}

	return r.generator.Close()
}

func (r *runner) checkEnrolledAgents() ([]kibana.Agent, error) {
	var agents []kibana.Agent
	enrolled, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return false, errors.New("SIGINT: cancel checking enrolled agents")
		}
		allAgents, err := r.options.KibanaClient.ListAgents()
		if err != nil {
			return false, fmt.Errorf("could not list agents: %w", err)
		}

		agents = filterAgents(allAgents)
		if len(agents) == 0 {
			return false, nil // selected agents are unavailable yet
		}

		return true, nil
	}, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("agent enrollment failed: %w", err)
	}
	if !enrolled {
		return nil, errors.New("no agent enrolled in time")
	}
	return agents, nil
}

func (r *runner) waitUntilBenchmarkFinishes() error {
	logger.Debug("checking for all data in data stream...")
	var benchTime *time.Timer
	if r.scenario.BenchmarkTimePeriod > 0 {
		benchTime = time.NewTimer(r.scenario.BenchmarkTimePeriod)
	}
	waitForDataTimeout := waitForDataDefaultTimeout
	if r.scenario.WaitForDataTimeout > 0 {
		waitForDataTimeout = r.scenario.WaitForDataTimeout
	}

	oldHits := 0
	_, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel waiting for policy assigned")
		}

		var err error
		hits, err := getTotalHits(r.options.ESAPI, r.runtimeDataStream)
		if hits == 0 {
			return false, err
		}

		ret := hits == oldHits
		if hits != oldHits {
			oldHits = hits
		}

		if benchTime != nil {
			select {
			case <-benchTime.C:
				return true, err
			default:
				return false, err
			}
		}

		return ret, err
	}, waitForDataTimeout)
	return err
}

func (r *runner) enrollAgents() error {
	agents, err := r.checkEnrolledAgents()
	if err != nil {
		return fmt.Errorf("can't check enrolled agents: %w", err)
	}

	handlers := make([]func() error, len(agents))
	for i, agent := range agents {
		origPolicy := kibana.Policy{
			ID:       agent.PolicyID,
			Revision: agent.PolicyRevision,
		}

		// Assign policy to agent
		handlers[i] = func() error {
			logger.Debug("reassigning original policy back to agent...")
			if err := r.options.KibanaClient.AssignPolicyToAgent(agent, origPolicy); err != nil {
				return fmt.Errorf("error reassigning original policy to agent %s: %w", agent.ID, err)
			}
			return nil
		}

		policyWithDataStream, err := r.options.KibanaClient.GetPolicy(r.benchPolicy.ID)
		if err != nil {
			return fmt.Errorf("could not read the policy with data stream: %w", err)
		}

		logger.Debug("assigning package data stream to agent...")
		if err := r.options.KibanaClient.AssignPolicyToAgent(agent, *policyWithDataStream); err != nil {
			return fmt.Errorf("could not assign policy to agent: %w", err)
		}
	}

	r.resetAgentPolicyHandler = func() error {
		var merr multierror.Error
		for _, h := range handlers {
			if err := h(); err != nil {
				merr = append(merr, err)
			}
		}
		if len(merr) == 0 {
			return nil
		}
		return merr
	}

	return nil
}

func getTotalHits(esapi *elasticsearch.API, dataStream string) (int, error) {
	resp, err := esapi.Count(
		esapi.Count.WithIndex(dataStream),
	)
	if err != nil {
		return 0, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

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

func filterAgents(allAgents []kibana.Agent) []kibana.Agent {
	var filtered []kibana.Agent
	for _, agent := range allAgents {
		if agent.PolicyRevision == 0 {
			// For some reason Kibana doesn't always return
			// a valid policy revision (eventually it will be present and valid)
			continue
		}

		// best effort to ignore fleet server agents
		switch {
		case agent.LocalMetadata.Host.Name == "docker-fleet-server",
			agent.PolicyID == "fleet-server-policy",
			agent.PolicyID == "Elastic Cloud agent policy":
			continue
		}
		filtered = append(filtered, agent)
	}
	return filtered
}

func waitUntilTrue(fn func() (bool, error), timeout time.Duration) (bool, error) {
	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()

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
		case <-timeoutTicker.C:
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
