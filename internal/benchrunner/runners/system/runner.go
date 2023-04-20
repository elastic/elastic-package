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
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
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

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() benchrunner.Type {
	return BenchType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return string(BenchType)
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
	r.ctxt.Test.RunID = createRunID()

	scenario, err := readConfig(r.options.PackageRootPath, r.options.BenchName, r.ctxt)
	if err != nil {
		return err
	}
	r.scenario = scenario

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
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

	if r.scenario.Corpora.Generator != nil {
		var err error
		r.generator, err = r.initializeGenerator()
		if err != nil {
			return fmt.Errorf("can't initialize generator: %w", err)
		}
	}

	policyTemplateName := r.scenario.PolicyTemplate
	if policyTemplateName == "" {
		policyTemplateName, err = findPolicyTemplateForInput(*pkgManifest, *dataStreamManifest, r.scenario.Input)
		if err != nil {
			return fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
	}

	policyTemplate, err := selectPolicyTemplateByName(pkgManifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return fmt.Errorf("failed to find the selected policy_template: %w", err)
	}

	kib, err := kibana.NewClient()
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	// Configure package (single data stream) via Ingest Manager APIs.
	logger.Debug("creating benchmark policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := kibana.Policy{
		Name:              fmt.Sprintf("ep-bench-%s-%s", r.options.BenchName, testTime),
		Description:       fmt.Sprintf("policy created by elastic-package for benchmark %s", r.options.BenchName),
		Namespace:         "ep",
		MonitoringEnabled: []string{"logs", "metrics"},
	}
	policy, err := kib.CreatePolicy(p)
	if err != nil {
		return fmt.Errorf("could not create benchmark policy: %w", err)
	}

	r.deletePolicyHandler = func() error {
		logger.Debug("deleting benchmark policy...")
		if err := kib.DeletePolicy(*policy); err != nil {
			return fmt.Errorf("error cleaning up benchmark policy: %w", err)
		}
		return nil
	}

	logger.Debug("adding package data stream to benchmark policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, policyTemplate, *dataStreamManifest, *r.scenario)
	if err := kib.AddPackageDataStreamToPolicy(ds); err != nil {
		return fmt.Errorf("could not add data stream config to policy: %w", err)
	}

	// Delete old data
	logger.Debug("deleting old data in data stream...")
	dataStream := fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		ds.Inputs[0].Streams[0].DataStream.Dataset,
		ds.Namespace,
	)

	r.runtimeDataStream = dataStream

	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
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

	agents, err := checkEnrolledAgents(kib)
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
			if err := kib.AssignPolicyToAgent(agent, origPolicy); err != nil {
				return fmt.Errorf("error reassigning original policy to agent %s: %w", agent.ID, err)
			}
			return nil
		}

		policyWithDataStream, err := kib.GetPolicy(policy.ID)
		if err != nil {
			return fmt.Errorf("could not read the policy with data stream: %w", err)
		}

		logger.Debug("assigning package data stream to agent...")
		if err := kib.AssignPolicyToAgent(agent, *policyWithDataStream); err != nil {
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

		ret := hits != oldHits
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

// findPolicyTemplateForInput returns the name of the policy_template that
// applies to the benchmarked input. An error is returned if no policy template
// matches or if multiple policy templates match and the response is ambiguous.
func findPolicyTemplateForInput(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if pkg.Type == "input" {
		return findPolicyTemplateForInputPackage(pkg, inputName)
	}
	return findPolicyTemplateForDataStream(pkg, ds, inputName)
}

func findPolicyTemplateForDataStream(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(ds.Streams) == 0 {
			return "", errors.New("no streams declared in data stream manifest")
		}
		inputName = ds.Streams[getDataStreamIndex(inputName, ds)].Input
	}

	var matchedPolicyTemplates []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		// Does this policy_template include this input type?
		if policyTemplate.FindInputByType(inputName) == nil {
			continue
		}

		// Does the policy_template apply to this data stream (when data streams are specified)?
		if len(policyTemplate.DataStreams) > 0 && !common.StringSliceContains(policyTemplate.DataStreams, ds.Name) {
			continue
		}

		matchedPolicyTemplates = append(matchedPolicyTemplates, policyTemplate.Name)
	}

	switch len(matchedPolicyTemplates) {
	case 1:
		return matchedPolicyTemplates[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found for data stream %q "+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", ds.Name, inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"were found that apply to data stream %q with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matchedPolicyTemplates, ", "), ds.Name, inputName)
	}
}

func findPolicyTemplateForInputPackage(pkg packages.PackageManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(pkg.PolicyTemplates) == 0 {
			return "", errors.New("no policy templates specified for input package")
		}
		inputName = pkg.PolicyTemplates[0].Input
	}

	var matched []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != inputName {
			continue
		}

		matched = append(matched, policyTemplate.Name)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found"+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"with input type %q: please "+
			"specify the 'policy_template' in the system benchmark config",
			strings.Join(matched, ", "), inputName)
	}
}

// getDataStreamIndex returns the index of the data stream whose input name
// matches. Otherwise it returns the 0.
func getDataStreamIndex(inputName string, ds packages.DataStreamManifest) int {
	for i, s := range ds.Streams {
		if s.Input == inputName {
			return i
		}
	}
	return 0
}

func getDataStreamPath(packageRoot, dataStream string) string {
	return filepath.Join(packageRoot, "data_stream", dataStream)
}

func selectPolicyTemplateByName(policies []packages.PolicyTemplate, name string) (packages.PolicyTemplate, error) {
	for _, policy := range policies {
		if policy.Name == name {
			return policy, nil
		}
	}
	return packages.PolicyTemplate{}, fmt.Errorf("policy template %q not found", name)
}

func createPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	scenario scenario,
) kibana.PackageDataStream {
	if pkg.Type == "input" {
		return createInputPackageDatastream(kibanaPolicy, pkg, policyTemplate, scenario)
	}
	return createIntegrationPackageDatastream(kibanaPolicy, pkg, policyTemplate, ds, scenario)
}

func createIntegrationPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	scenario scenario,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, ds.Name),
		Namespace: "ep",
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
		Inputs: []kibana.Input{
			{
				PolicyTemplate: policyTemplate.Name,
				Enabled:        true,
			},
		},
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	stream := ds.Streams[getDataStreamIndex(scenario.Input, ds)]
	streamInput := stream.Input
	r.Inputs[0].Type = streamInput

	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    ds.Type,
				Dataset: getDataStreamDataset(pkg, ds),
			},
		},
	}

	// Add dataStream-level vars
	streams[0].Vars = setKibanaVariables(stream.Vars, scenario.DataStream.Vars)
	r.Inputs[0].Streams = streams

	// Add package-level vars
	var inputVars []packages.Variable
	input := policyTemplate.FindInputByType(streamInput)
	if input != nil {
		// copy package-level vars into each input
		inputVars = append(inputVars, input.Vars...)
		inputVars = append(inputVars, pkg.Vars...)
	}

	r.Inputs[0].Vars = setKibanaVariables(inputVars, scenario.Vars)

	return r
}

func createInputPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	scenario scenario,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, policyTemplate.Name),
		Namespace: "ep",
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version
	r.Inputs = []kibana.Input{
		{
			PolicyTemplate: policyTemplate.Name,
			Enabled:        true,
			Vars:           kibana.Vars{},
		},
	}

	streamInput := policyTemplate.Input
	r.Inputs[0].Type = streamInput

	dataset := fmt.Sprintf("%s.%s", pkg.Name, policyTemplate.Name)
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, policyTemplate.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    policyTemplate.Type,
				Dataset: dataset,
			},
		},
	}

	// Add policyTemplate-level vars.
	vars := setKibanaVariables(policyTemplate.Vars, scenario.Vars)
	if _, found := vars["data_stream.dataset"]; !found {
		var value packages.VarValue
		_ = value.Unpack(dataset)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	streams[0].Vars = vars
	r.Inputs[0].Streams = streams
	return r
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = packages.VarValue{}
			_ = val.Unpack(value)
		}

		vars[definition.Name] = kibana.Var{
			Type:  definition.Type,
			Value: val,
		}
	}
	return vars
}

func getDataStreamDataset(pkg packages.PackageManifest, ds packages.DataStreamManifest) string {
	if len(ds.Dataset) > 0 {
		return ds.Dataset
	}
	return fmt.Sprintf("%s.%s", pkg.Name, ds.Name)
}

func checkEnrolledAgents(client *kibana.Client) ([]kibana.Agent, error) {
	var agents []kibana.Agent
	enrolled, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return false, errors.New("SIGINT: cancel checking enrolled agents")
		}
		allAgents, err := client.ListAgents()
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

	retryTicker := time.NewTicker(1 * time.Second)
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

func (r *runner) getTotalHits(dataStream string) (int, error) {
	resp, err := r.options.API.Search(
		r.options.API.Search.WithIndex(dataStream),
		r.options.API.Search.WithSort("@timestamp:asc"),
		r.options.API.Search.WithSize(elasticsearchQuerySize),
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

func deleteDataStreamDocs(api *elasticsearch.API, dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	_, err := api.DeleteByQuery([]string{dataStream}, body)
	if err != nil {
		return err
	}

	return nil
}

func createRunID() string {
	return fmt.Sprintf("%d", rand.Intn(runMaxID-runMinID)+runMinID)
}

func (r *runner) initializeGenerator() (genlib.Generator, error) {
	totSizeInBytes, err := humanize.ParseBytes(r.scenario.Corpora.Generator.Size)
	if err != nil {
		return nil, err
	}

	tplPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.TemplatePath))
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		return nil, fmt.Errorf("can't open template file %s: %w", tplPath, err)
	}

	configPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.ConfigPath))
	config, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("can't open config file %s: %w", configPath, err)
	}

	fieldsPath := filepath.Clean(filepath.Join(devPath, r.scenario.Corpora.Generator.FieldsPath))
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
