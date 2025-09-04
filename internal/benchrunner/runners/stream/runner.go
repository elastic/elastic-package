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

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/wait"
)

const numberOfEvents = 100

type runner struct {
	options   Options
	scenarios map[string]*scenario

	svcInfo            servicedeployer.ServiceInfo
	runtimeDataStreams map[string]string
	generators         map[string]genlib.Generator
	backFillGenerators map[string]genlib.Generator

	// Execution order of following handlers is defined in runner.TearDown() method.
	removePackageHandler  func(context.Context) error
	wipeDataStreamHandler func(context.Context) error
}

func NewStreamBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

func (r *runner) SetUp(ctx context.Context) error {
	return r.setUp(ctx)
}

func StaticValidation(ctx context.Context, opts Options, dataStreamName string) (bool, error) {
	runner := runner{options: opts}
	err := runner.initialize()
	if err != nil {
		return false, err
	}
	hasBenchmark, err := runner.validateScenario(ctx, dataStreamName)
	return hasBenchmark, err
}

// Run runs the system benchmarks defined under the given folder
func (r *runner) Run(ctx context.Context) (reporters.Reportable, error) {
	return nil, r.run(ctx)
}

func (r *runner) TearDown(ctx context.Context) error {
	if !r.options.PerformCleanup {
		r.removePackageHandler = nil
		r.wipeDataStreamHandler = nil
		return nil
	}

	// Avoid cancellations during cleanup.
	cleanupCtx := context.WithoutCancel(ctx)

	var merr multierror.Error

	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(cleanupCtx); err != nil {
			merr = append(merr, err)
		}
		r.removePackageHandler = nil
	}

	if r.wipeDataStreamHandler != nil {
		if err := r.wipeDataStreamHandler(cleanupCtx); err != nil {
			merr = append(merr, err)
		}
		r.wipeDataStreamHandler = nil
	}

	if len(merr) == 0 {
		return nil
	}
	return merr
}

func (r *runner) initialize() error {
	r.generators = make(map[string]genlib.Generator)
	r.backFillGenerators = make(map[string]genlib.Generator)

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
	}

	scenarios, err := readScenarios(r.options.PackageRootPath, r.options.BenchName, pkgManifest.Name, pkgManifest.Version)
	if err != nil {
		return err
	}
	r.scenarios = scenarios

	return nil
}

func (r *runner) validateScenario(ctx context.Context, dataStreamName string) (bool, error) {
	for scenarioName, scenario := range r.scenarios {
		if scenario.DataStream.Name != dataStreamName {
			continue
		}
		generator, _, err := r.createGenerator(ctx, scenarioName, scenario)
		if err != nil {
			return true, err
		}
		for i := 0; i < numberOfEvents; i++ {
			buf := bytes.NewBufferString("")
			err := generator.Emit(buf)
			if err != nil {
				return true, fmt.Errorf("[%s] error while generating event: %w", scenarioName, err)
			}
			// check whether the generated event is valid json
			var event map[string]any
			err = json.Unmarshal(buf.Bytes(), &event)
			if err != nil {
				return true, fmt.Errorf("[%s] failed to unmarshal json event: %w, generated output: %s", scenarioName, err, buf.String())
			}
		}
		return true, nil
	}

	return false, nil
}

func (r *runner) setUp(ctx context.Context) error {
	err := r.initialize()
	if err != nil {
		return err
	}

	err = r.collectGenerators(ctx)
	if err != nil {
		return fmt.Errorf("can't initialize generator: %w", err)
	}

	r.runtimeDataStreams = make(map[string]string)

	r.svcInfo.Test.RunID = common.NewRunID()

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed: %w", err)
	}

	if err = r.installPackage(ctx); err != nil {
		return fmt.Errorf("error installing package: %w", err)
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

	if err := r.wipeDataStreamsOnSetup(ctx); err != nil {
		return fmt.Errorf("error cleaning up old data in data streams: %w", err)
	}

	cleared, err := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		totalHits := 0
		for _, runtimeDataStream := range r.runtimeDataStreams {
			hits, err := common.CountDocsInDataStream(ctx, r.options.ESAPI, runtimeDataStream)
			if err != nil {
				return false, err
			}
			totalHits += hits
		}
		return totalHits == 0, nil
	}, 5*time.Second, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return err
	}

	return nil
}

func (r *runner) wipeDataStreamsOnSetup(ctx context.Context) error {
	// Delete old data
	logger.Debug("deleting old data in data stream...")
	r.wipeDataStreamHandler = func(ctx context.Context) error {
		logger.Debugf("deleting data in data stream...")
		for _, runtimeDataStream := range r.runtimeDataStreams {
			if err := r.deleteDataStreamDocs(ctx, runtimeDataStream); err != nil {
				return fmt.Errorf("error deleting data in data stream: %w", err)
			}
		}
		return nil
	}

	for _, runtimeDataStream := range r.runtimeDataStreams {
		if err := r.deleteDataStreamDocs(ctx, runtimeDataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
	}

	return nil
}

func (r *runner) installPackage(ctx context.Context) error {
	return r.installPackageFromPackageRoot(ctx)
}

func (r *runner) installPackageFromPackageRoot(ctx context.Context) error {
	logger.Debug("Installing package...")
	installer, err := installer.NewForPackage(ctx, installer.Options{
		Kibana:         r.options.KibanaClient,
		RootPath:       r.options.PackageRootPath,
		SkipValidation: true,
	})

	if err != nil {
		return fmt.Errorf("failed to initialize package installer: %w", err)
	}

	_, err = installer.Install(ctx)
	if err != nil {
		return fmt.Errorf("failed to install package: %w", err)
	}

	r.removePackageHandler = func(ctx context.Context) error {
		if err := installer.Uninstall(ctx); err != nil {
			return fmt.Errorf("error removing benchmark package: %w", err)
		}

		return nil
	}

	return nil
}

func (r *runner) deleteDataStreamDocs(ctx context.Context, dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	resp, err := r.options.ESAPI.DeleteByQuery([]string{dataStream}, body,
		r.options.ESAPI.DeleteByQuery.WithContext(ctx),
	)
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
func (r *runner) collectGenerators(ctx context.Context) error {
	for scenarioName, scenario := range r.scenarios {
		generator, backfillGenerator, err := r.createGenerator(ctx, scenarioName, scenario)
		if err != nil {
			return err
		}

		r.generators[scenarioName] = generator

		if backfillGenerator != nil {
			r.backFillGenerators[scenarioName] = backfillGenerator
		}
	}

	return nil
}

func (r *runner) createGenerator(ctx context.Context, scenarioName string, scenario *scenario) (genlib.Generator, genlib.Generator, error) {
	config, err := r.getGeneratorConfig(scenario)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to obtain generator config for scenario %q: %w", scenarioName, err)
	}

	fields, err := r.getGeneratorFields(ctx, scenario)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to obtain fields from generator for scenario %q: %w", scenarioName, err)
	}

	tpl, err := r.getGeneratorTemplate(scenario)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to obtain template from for scenario %q: %w", scenarioName, err)
	}

	genlib.InitGeneratorTimeNow(time.Now())
	genlib.InitGeneratorRandSeed(time.Now().UnixNano())

	generator, err := r.initializeGenerator(tpl, *config, fields, scenario, 0, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize backfill generator for scenario %q: %w", scenarioName, err)
	}

	if r.options.BackFill >= 0 {
		return generator, nil, nil
	}

	// backfill is a negative duration, make it positive, find how many periods in the backfill and multiply by events for periodk
	totEvents := uint64((-1*r.options.BackFill)/r.options.PeriodDuration) * r.options.EventsPerPeriod

	backFillGenerator, err := r.initializeGenerator(tpl, *config, fields, scenario, r.options.BackFill, totEvents)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize backfill generator for scenario %q: %w", scenarioName, err)
	}

	return generator, backFillGenerator, nil
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

func (r *runner) getGeneratorFields(ctx context.Context, scenario *scenario) (fields.Fields, error) {
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

	fields, err := fields.LoadFieldsWithTemplateFromString(ctx, string(data))
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

func (r *runner) performBulkRequest(ctx context.Context, bulkRequest string) error {
	resp, err := r.options.ESAPI.Bulk(strings.NewReader(bulkRequest),
		r.options.ESAPI.Bulk.WithContext(ctx),
	)

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

func (r *runner) run(ctx context.Context) error {
	logger.Debug("streaming data...")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errC := make(chan error)
	defer close(errC)

	var wg sync.WaitGroup
	defer wg.Wait()

	for scenarioName := range r.generators {
		wg.Add(1)
		go func(scenarioName string) {
			defer wg.Done()
			err := r.runStreamGenerator(ctx, scenarioName)
			if err != nil {
				errC <- err
			}
		}(scenarioName)
	}

	for scenarioName := range r.backFillGenerators {
		wg.Add(1)
		go func(scenarioName string) {
			defer wg.Done()
			err := r.runBackfillGenerator(ctx, scenarioName)
			if err != nil {
				errC <- err
			}
		}(scenarioName)
	}

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errC:
		cancel()
	}
	// Ensure no goroutine is blocked sending errors.
	go func() {
		for range errC {
		}
	}()
	return err
}

func (r *runner) runStreamGenerator(ctx context.Context, scenarioName string) error {
	generator := r.generators[scenarioName]
	indexName := r.runtimeDataStreams[scenarioName]

	ticker := time.NewTicker(r.options.PeriodDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		logger.Debugf("bulk request of %d events on %s...", r.options.EventsPerPeriod, indexName)
		var bulkBodyBuilder strings.Builder
		buf := bytes.NewBufferString("")
		for i := uint64(0); i < r.options.EventsPerPeriod; i++ {
			var err error
			bulkBodyBuilder, err = r.collectBulkRequestBody(indexName, scenarioName, buf, generator, bulkBodyBuilder)
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				return fmt.Errorf("error while generating event for streaming: %w", err)
			}
		}

		err := r.performBulkRequest(ctx, bulkBodyBuilder.String())
		if err != nil {
			return fmt.Errorf("error performing bulk request: %w", err)
		}
	}
}

func (r *runner) runBackfillGenerator(ctx context.Context, scenarioName string) error {
	var bulkBodyBuilder strings.Builder
	generator := r.backFillGenerators[scenarioName]
	indexName := r.runtimeDataStreams[scenarioName]
	logger.Debugf("bulk request of %s backfill events on %s...", r.options.BackFill.String(), indexName)
	buf := bytes.NewBufferString("")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var err error
		bulkBodyBuilder, err = r.collectBulkRequestBody(indexName, scenarioName, buf, generator, bulkBodyBuilder)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("error while generating event for streaming: %w", err)
		}
	}

	return r.performBulkRequest(ctx, bulkBodyBuilder.String())
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
	m.Info.RunID = r.svcInfo.Test.RunID
	e["benchmark_metadata"] = m
	return e
}
