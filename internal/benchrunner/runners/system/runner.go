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
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"
	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/system/servicedeployer"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
)

const (
	runMaxID = 99999
	runMinID = 10000

	// Maximum number of events to query.
	elasticsearchQuerySize = 500

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

	generator         genlib.Generator
	ctxt              servicedeployer.ServiceContext
	runtimeDataStream string

	// Execution order of following handlers is defined in runner.TearDown() method.
	deletePolicyHandler     func() error
	resetAgentPolicyHandler func() error
	shutdownServiceHandler  func() error
	wipeDataStreamHandler   func() error
}

func NewSystemBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

func (r *runner) SetUp() error {
	return r.setUp()
}

// Run runs the system tests defined under the given folder
func (r *runner) Run() (reporters.Reportable, error) {
	return r.run()
}

func (r *runner) TearDown() error {
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

	dataStream := fmt.Sprintf(
		"%s-%s.%s-%s",
		dataStreamManifest.Type,
		pkgManifest.Name,
		dataStreamManifest.Name,
		policy.Namespace,
	)

	r.runtimeDataStream = dataStream

	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := r.deleteDataStreamDocs(dataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	if err := r.deleteDataStreamDocs(dataStream); err != nil {
		return fmt.Errorf("error deleting old data in data stream: %s: %w", dataStream, err)
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel clearing data")
		}

		hits, err := r.getTotalHits(dataStream)
		return hits == 0, err
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return err
	}

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

		policyWithDataStream, err := r.options.KibanaClient.GetPolicy(policy.ID)
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

	if r.generator != nil {
		logger.Debugf("generating corpus data to %s...", r.ctxt.Logs.Folder.Local)
		if err := r.runGenerator(r.ctxt.Logs.Folder.Local); err != nil {
			return nil, fmt.Errorf("can't generate benchmarks data corpus for data stream: %w", err)
		}
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if r.scenario.Corpora.InputService != nil && r.scenario.Corpora.InputService.Signal != "" {
		if err = service.Signal(r.scenario.Corpora.InputService.Signal); err != nil {
			return nil, fmt.Errorf("failed to notify test service: %w", err)
		}
	}

	logger.Debug("checking for all data in data stream...")
	oldHits := 0
	_, err = waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel waiting for policy assigned")
		}

		var err error
		hits, err := r.getTotalHits(r.runtimeDataStream)
		if hits == 0 {
			return false, err
		}

		ret := hits == oldHits
		if hits != oldHits {
			oldHits = hits
		}

		return ret, err
	}, waitForDataDefaultTimeout)
	if err != nil {
		return nil, err
	}

	// get metrics

	// generate report

	return nil, nil
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
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := kibana.Policy{
		Name:              fmt.Sprintf("ep-bench-%s-%s", r.options.BenchName, testTime),
		Description:       fmt.Sprintf("policy created by elastic-package for benchmark %s", r.options.BenchName),
		Namespace:         "ep",
		MonitoringEnabled: []string{"logs", "metrics"},
	}

	var out *kibana.Output
	if r.options.MetricstoreESURL != "" {
		o := kibana.Output{
			Name:                "Benchmark Metricstore",
			Type:                "elasticsearch",
			IsDefault:           false,
			IsDefaultMonitoring: true,
			Hosts:               []string{r.options.MetricstoreESURL},
		}

		var err error
		out, err = r.options.KibanaClient.CreateOutput(o)
		if err != nil {
			return nil, err
		}
		p.MonitoringOutputID = out.ID
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

		if out != nil {
			logger.Debug("deleting benchmark metricstore output...")
			if err := r.options.KibanaClient.DeleteOutput(*out); err != nil {
				merr = append(merr, fmt.Errorf("error cleaning up benchmark policy: %w", err))
			}
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

	pp := kibana.PackagePolicy{
		Namespace: "ep",
		PolicyID:  p.ID,
		Vars:      r.scenario.Vars,
		Force:     true,
		Inputs: map[string]kibana.PackagePolicyInput{
			fmt.Sprintf("%s-%s", r.scenario.DataStream.Name, r.scenario.Input): {
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

	tplPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Template.Path))
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		return nil, fmt.Errorf("can't open template file %s: %w", tplPath, err)
	}

	configPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Config.Path))
	config, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("can't open config file %s: %w", configPath, err)
	}

	fieldsPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.Fields.Path))
	fieldsBytes, err := os.ReadFile(fieldsPath)
	if err != nil {
		return nil, fmt.Errorf("can't open fields file %s: %w", tplPath, err)
	}

	fields, err := fields.LoadFieldsWithTemplateFromString(context.Background(), string(fieldsBytes))
	if err != nil {
		return nil, fmt.Errorf("could not load fields yaml: %w", err)
	}

	generator, err := genlib.NewGeneratorWithCustomTemplate(tpl, config, fields, totSizeInBytes)
	if err != nil {
		return nil, err
	}

	return generator, nil
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
		event := bytes.ReplaceAll(buf.Bytes(), []byte("\n"), []byte(""))
		if _, err = f.Write(event); err != nil {
			return err
		}

		if _, err = f.Write([]byte("\n")); err != nil {
			return err
		}

		buf.Reset()
		corpusDocsCount += 1
	}

	return r.generator.Close()
}

func (r *runner) getTotalHits(dataStream string) (int, error) {
	resp, err := r.options.ESAPI.Search(
		r.options.ESAPI.Search.WithIndex(dataStream),
		r.options.ESAPI.Search.WithSort("@timestamp:asc"),
		r.options.ESAPI.Search.WithSize(elasticsearchQuerySize),
	)
	if err != nil {
		return 0, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

	var results struct {
		Hits struct {
			Total struct {
				Value int
			}
		}
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, fmt.Errorf("could not decode search results response: %w", err)
	}

	numHits := results.Hits.Total.Value
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	return numHits, nil
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

func filterAgents(allAgents []kibana.Agent) []kibana.Agent {
	var filtered []kibana.Agent
	for _, agent := range allAgents {
		if agent.PolicyRevision == 0 {
			continue // For some reason Kibana doesn't always return a valid policy revision (eventually it will be present and valid)
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
	return fmt.Sprintf("%d", rand.Intn(runMaxID-runMinID)+runMinID)
}

func getDataStreamPath(packageRoot, dataStream string) string {
	return filepath.Join(packageRoot, "data_stream", dataStream)
}
