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
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/requiredinputs"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	defaultLogsDBColumnarComponentTemplateName = "logs@custom"
	logsDBColumnarIndexMode                    = "logsdb_columnar"
	logsDBColumnarStateFileName                = "logsdb-columnar-template.json"
)

// logsDBColumnarDocValuesDynamicTemplates are prepended to @custom so they match
// before ecs@mappings templates that set doc_values:false. This is a test-only
// workaround; ECS still needs a product solution for logsdb_columnar.
var logsDBColumnarDocValuesDynamicTemplates = []map[string]any{
	{
		"event_original_logsdb_columnar_workaround": map[string]any{
			"path_match": []any{"event.original", "*event.original", "*gen_ai.agent.description"},
			"mapping": map[string]any{
				"type":       "keyword",
				"index":      false,
				"doc_values": true,
			},
		},
	},
	{
		"x509_public_key_exponent_logsdb_columnar_workaround": map[string]any{
			"path_match": "*.x509.public_key_exponent",
			"mapping": map[string]any{
				"type":       "long",
				"index":      false,
				"doc_values": true,
			},
		},
	},
}

type logsDBColumnarTemplateState struct {
	Templates         map[string]logsDBColumnarTemplateSnapshot `json:"templates,omitempty"`
	PackageTemplates  map[string]logsDBColumnarTemplateSnapshot `json:"package_templates,omitempty"`
	Existed           bool                                      `json:"existed,omitempty"`
	ComponentTemplate json.RawMessage                           `json:"component_template,omitempty"`
}

type logsDBColumnarTemplateSnapshot struct {
	Existed           bool            `json:"existed"`
	ComponentTemplate json.RawMessage `json:"component_template,omitempty"`
}

type runner struct {
	profile        *profile.Profile
	repositoryRoot *os.Root
	packageRoot    string
	kibanaClient   *kibana.Client
	esAPI          *elasticsearch.API
	esClient       *elasticsearch.Client
	schemaURLs     fields.SchemaURLs

	dataStreams          []string
	serviceVariant       string
	overrideAgentVersion string

	globalTestConfig   testrunner.GlobalRunnerTestConfig
	failOnMissingTests bool
	deferCleanup       time.Duration
	generateTestResult bool
	withCoverage       bool
	coverageType       string
	logsDBColumnar     bool

	configFilePath string
	runSetup       bool
	runTearDown    bool
	runTestsOnly   bool

	resourcesManager        *resources.Manager
	serviceStateFilePath    string
	logsDBColumnarState     *logsDBColumnarTemplateState
	logsDBColumnarStatePath string
	requiredInputsResolver  requiredinputs.Resolver
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type SystemTestRunnerOptions struct {
	Profile              *profile.Profile
	PackageRoot          string
	RepositoryRoot       *os.Root
	KibanaClient         *kibana.Client
	API                  *elasticsearch.API
	OverrideAgentVersion string
	SchemaURLs           fields.SchemaURLs

	// FIXME: Keeping Elasticsearch client to be able to do low-level requests for parameters not supported yet by the API.
	ESClient *elasticsearch.Client

	DataStreams    []string
	ServiceVariant string

	RunSetup       bool
	RunTearDown    bool
	RunTestsOnly   bool
	ConfigFilePath string

	GlobalTestConfig testrunner.GlobalRunnerTestConfig

	FailOnMissingTests     bool
	GenerateTestResult     bool
	DeferCleanup           time.Duration
	WithCoverage           bool
	CoverageType           string
	RequiredInputsResolver requiredinputs.Resolver
	LogsDBColumnar         bool
}

func NewSystemTestRunner(options SystemTestRunnerOptions) *runner {
	r := runner{
		packageRoot:            options.PackageRoot,
		kibanaClient:           options.KibanaClient,
		esAPI:                  options.API,
		esClient:               options.ESClient,
		profile:                options.Profile,
		schemaURLs:             options.SchemaURLs,
		dataStreams:            options.DataStreams,
		serviceVariant:         options.ServiceVariant,
		configFilePath:         options.ConfigFilePath,
		runSetup:               options.RunSetup,
		runTestsOnly:           options.RunTestsOnly,
		runTearDown:            options.RunTearDown,
		failOnMissingTests:     options.FailOnMissingTests,
		generateTestResult:     options.GenerateTestResult,
		deferCleanup:           options.DeferCleanup,
		globalTestConfig:       options.GlobalTestConfig,
		withCoverage:           options.WithCoverage,
		coverageType:           options.CoverageType,
		logsDBColumnar:         options.LogsDBColumnar,
		repositoryRoot:         options.RepositoryRoot,
		overrideAgentVersion:   options.OverrideAgentVersion,
		requiredInputsResolver: options.RequiredInputsResolver,
	}

	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})

	r.serviceStateFilePath = filepath.Join(stateFolderPath(r.profile.ProfilePath), serviceStateFileName)
	r.logsDBColumnarStatePath = filepath.Join(stateFolderPath(r.profile.ProfilePath), logsDBColumnarStateFileName)
	return &r
}

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context) error {
	if r.runTearDown {
		logger.Debug("Skip installing package")
		return nil
	}

	// Install the package before creating the policy, so we control exactly what is being
	// installed.
	logger.Info("Installing package...")
	resourcesOptions := resourcesOptions{
		// Install it unless we are running the tear down only.
		installedPackage: !r.runTearDown,
	}
	_, err := r.resourcesManager.ApplyCtx(ctx, r.resources(resourcesOptions))
	if err != nil {
		return fmt.Errorf("can't install the package: %w", err)
	}

	// Configure logsdb_columnar after install so @package mappings exist. Mode and
	// doc_values overrides must be applied together: ES rejects columnar mode when
	// composed mappings still contain doc_values:false fields.
	if r.logsDBColumnar {
		if err := r.ensureLogsDBColumnarTemplate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// TearDownRunner cleans up any global test runner resources. It must be called
// after the test runner has finished executing all its tests.
func (r *runner) TearDownRunner(ctx context.Context) error {
	logger.Info("Uninstalling package...")
	resourcesOptions := resourcesOptions{
		// Keep it installed only if we were running setup, or tests only.
		installedPackage: r.runSetup || r.runTestsOnly,
	}
	_, resourceErr := r.resourcesManager.ApplyCtx(ctx, r.resources(resourcesOptions))

	var templateErr error
	if r.logsDBColumnar && !r.runSetup && !r.runTestsOnly {
		templateErr = r.restoreLogsDBColumnarTemplate(ctx)
	}

	if resourceErr != nil && templateErr != nil {
		return fmt.Errorf("failed to clean system runner resources: %w", errors.Join(resourceErr, templateErr))
	}
	if resourceErr != nil {
		return resourceErr
	}
	if templateErr != nil {
		return templateErr
	}
	return nil
}

func (r *runner) ensureLogsDBColumnarTemplate(ctx context.Context) error {
	state, err := r.loadLogsDBColumnarState()
	if err != nil {
		return err
	}
	if state != nil {
		logger.Debug("LogsDB Columnar component template already configured in prior setup")
		return nil
	}

	supported, err := r.hasColumnarIndexModeCapability(ctx)
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf("logsdb columnar is not supported by this cluster: required create-index capability %q is unavailable", "columnar_index_modes")
	}

	templateNames, err := r.logsDBColumnarTemplateNames()
	if err != nil {
		return err
	}
	if len(templateNames) == 0 {
		logger.Debug("No logs data streams selected for LogsDB Columnar")
		return nil
	}

	state = &logsDBColumnarTemplateState{
		Templates:        make(map[string]logsDBColumnarTemplateSnapshot, len(templateNames)),
		PackageTemplates: make(map[string]logsDBColumnarTemplateSnapshot),
	}

	// Strip subobjects from every logs @package template in the package, not only
	// selected streams. Sibling streams can still be created under
	// cluster.logsdb_columnar.enabled even when only one stream is under test.
	packageTemplateNames, err := r.logsDBColumnarPackageTemplateNames()
	if err != nil {
		return err
	}
	packageTemplates := make(map[string]json.RawMessage, len(packageTemplateNames))
	for _, packageTemplateName := range packageTemplateNames {
		packageTemplate, packageExists, err := r.getComponentTemplate(ctx, packageTemplateName)
		if err != nil {
			return err
		}
		if !packageExists {
			continue
		}
		// Columnar mode rejects explicit subobjects mapping params (it implies
		// subobjects disabled). Strip them from @package before enabling mode.
		strippedPackageTemplate, stripped, err := stripSubobjectsFromComponentTemplate(packageTemplate)
		if err != nil {
			return fmt.Errorf("failed to strip subobjects from %q: %w", packageTemplateName, err)
		}
		if stripped {
			if err := r.putComponentTemplate(ctx, packageTemplateName, strippedPackageTemplate); err != nil {
				return err
			}
			state.PackageTemplates[packageTemplateName] = logsDBColumnarTemplateSnapshot{
				Existed:           true,
				ComponentTemplate: packageTemplate,
			}
			logger.Debugf("Stripped subobjects from %s for logsdb_columnar testing", packageTemplateName)
			packageTemplate = strippedPackageTemplate
		}
		packageTemplates[packageTemplateName] = packageTemplate
	}

	for _, templateName := range templateNames {
		currentTemplate, exists, err := r.getComponentTemplate(ctx, templateName)
		if err != nil {
			return err
		}

		overrides := logsDBColumnarPropertyOverrides()
		packageTemplateName := packageComponentTemplateName(templateName)
		if packageTemplate, ok := packageTemplates[packageTemplateName]; ok {
			for path, mapping := range collectDocValuesDisabledFieldOverrides(packageTemplate) {
				overrides[path] = mapping
			}
		}

		payload, err := buildLogsDBColumnarTemplatePayload(currentTemplate, exists, overrides, logsDBColumnarDocValuesDynamicTemplates)
		if err != nil {
			return err
		}
		if err := r.putComponentTemplate(ctx, templateName, payload); err != nil {
			return err
		}
		state.Templates[templateName] = logsDBColumnarTemplateSnapshot{
			Existed:           exists,
			ComponentTemplate: currentTemplate,
		}
		logger.Debugf("Configured %s with index.mode=%s and %d doc_values overrides for system tests", templateName, logsDBColumnarIndexMode, len(overrides))
	}

	if err := r.saveLogsDBColumnarState(state); err != nil {
		return err
	}
	r.logsDBColumnarState = state
	return nil
}

func (r *runner) restoreLogsDBColumnarTemplate(ctx context.Context) error {
	state, err := r.loadLogsDBColumnarState()
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}

	// Remove columnar mode from @custom first so @package can regain subobjects.
	for templateName, snapshot := range state.Templates {
		if snapshot.Existed {
			if err := r.putComponentTemplate(ctx, templateName, snapshot.ComponentTemplate); err != nil {
				return err
			}
			logger.Debugf("Restored previous %s component template", templateName)
		} else {
			if err := r.deleteComponentTemplate(ctx, templateName); err != nil {
				return err
			}
			logger.Debugf("Removed temporary %s component template", templateName)
		}
	}

	for templateName, snapshot := range state.PackageTemplates {
		if err := r.putComponentTemplate(ctx, templateName, snapshot.ComponentTemplate); err != nil {
			return err
		}
		logger.Debugf("Restored previous %s component template", templateName)
	}

	r.logsDBColumnarState = nil
	if err := os.Remove(r.logsDBColumnarStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove logsdb columnar state file: %w", err)
	}
	return nil
}

func logsDBColumnarPropertyOverrides() map[string]map[string]any {
	return map[string]map[string]any{
		// ECS dynamic templates leave event.original without doc values.
		"event.original": {
			"type":       "keyword",
			"index":      false,
			"doc_values": true,
		},
	}
}

func packageComponentTemplateName(customTemplateName string) string {
	if !strings.HasSuffix(customTemplateName, "@custom") {
		return ""
	}
	return strings.TrimSuffix(customTemplateName, "@custom") + "@package"
}

func (r *runner) hasColumnarIndexModeCapability(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/_capabilities", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create capabilities request: %w", err)
	}

	query := req.URL.Query()
	query.Set("method", http.MethodPut)
	query.Set("path", "/{index}")
	query.Set("capabilities", "columnar_index_modes")
	req.URL.RawQuery = query.Encode()

	resp, err := r.esClient.Transport.Perform(req)
	if err != nil {
		return false, fmt.Errorf("failed to query Elasticsearch capabilities: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 == 4 {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected status querying Elasticsearch capabilities: %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Supported bool `json:"supported"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return false, fmt.Errorf("failed to decode Elasticsearch capabilities response: %w", err)
	}
	return response.Supported, nil
}

func (r *runner) getComponentTemplate(ctx context.Context, templateName string) (json.RawMessage, bool, error) {
	resp, err := r.esAPI.Cluster.GetComponentTemplate(
		r.esAPI.Cluster.GetComponentTemplate.WithContext(ctx),
		r.esAPI.Cluster.GetComponentTemplate.WithName(templateName),
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to query component template %q: %w", templateName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.IsError() {
		return nil, false, fmt.Errorf("failed to query component template %q: %s", templateName, resp.String())
	}

	var response struct {
		ComponentTemplates []struct {
			Name              string          `json:"name"`
			ComponentTemplate json.RawMessage `json:"component_template"`
		} `json:"component_templates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, false, fmt.Errorf("failed to decode component template %q response: %w", templateName, err)
	}

	if len(response.ComponentTemplates) == 0 {
		return nil, false, nil
	}
	return response.ComponentTemplates[0].ComponentTemplate, true, nil
}

func (r *runner) putComponentTemplate(ctx context.Context, templateName string, payload []byte) error {
	payload, err := sanitizeComponentTemplateForPut(payload)
	if err != nil {
		return fmt.Errorf("failed to prepare component template %q for put: %w", templateName, err)
	}

	resp, err := r.esAPI.Cluster.PutComponentTemplate(
		templateName,
		bytes.NewReader(payload),
		r.esAPI.Cluster.PutComponentTemplate.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to put component template %q: %w", templateName, err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to put component template %q: %s", templateName, resp.String())
	}
	return nil
}

func (r *runner) deleteComponentTemplate(ctx context.Context, templateName string) error {
	resp, err := r.esAPI.Cluster.DeleteComponentTemplate(
		templateName,
		r.esAPI.Cluster.DeleteComponentTemplate.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to delete component template %q: %w", templateName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.IsError() {
		return fmt.Errorf("failed to delete component template %q: %s", templateName, resp.String())
	}
	return nil
}

func (r *runner) loadLogsDBColumnarState() (*logsDBColumnarTemplateState, error) {
	if r.logsDBColumnarState != nil {
		return r.logsDBColumnarState, nil
	}

	content, err := os.ReadFile(r.logsDBColumnarStatePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read logsdb columnar state file: %w", err)
	}

	var state logsDBColumnarTemplateState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("failed to decode logsdb columnar state file: %w", err)
	}
	if len(state.Templates) == 0 {
		if state.Existed || len(state.ComponentTemplate) > 0 {
			state.Templates = map[string]logsDBColumnarTemplateSnapshot{
				defaultLogsDBColumnarComponentTemplateName: {
					Existed:           state.Existed,
					ComponentTemplate: state.ComponentTemplate,
				},
			}
		}
	}
	r.logsDBColumnarState = &state
	return &state, nil
}

func (r *runner) saveLogsDBColumnarState(state *logsDBColumnarTemplateState) error {
	if err := os.MkdirAll(filepath.Dir(r.logsDBColumnarStatePath), 0755); err != nil {
		return fmt.Errorf("failed to create logsdb columnar state directory: %w", err)
	}

	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode logsdb columnar state: %w", err)
	}
	if err := os.WriteFile(r.logsDBColumnarStatePath, content, 0644); err != nil {
		return fmt.Errorf("failed to persist logsdb columnar state: %w", err)
	}
	return nil
}

func (r *runner) logsDBColumnarTemplateNames() ([]string, error) {
	return r.logsDBColumnarCustomTemplateNames(true)
}

// logsDBColumnarPackageTemplateNames returns @package component template names
// for every logs data stream in the package.
func (r *runner) logsDBColumnarPackageTemplateNames() ([]string, error) {
	customNames, err := r.logsDBColumnarCustomTemplateNames(false)
	if err != nil {
		return nil, err
	}
	packageNames := make([]string, 0, len(customNames))
	for _, customName := range customNames {
		if packageName := packageComponentTemplateName(customName); packageName != "" {
			packageNames = append(packageNames, packageName)
		}
	}
	return packageNames, nil
}

func (r *runner) logsDBColumnarCustomTemplateNames(selectedOnly bool) ([]string, error) {
	if r.packageRoot == "" {
		if selectedOnly {
			return []string{defaultLogsDBColumnarComponentTemplateName}, nil
		}
		return nil, nil
	}

	packageManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed (path: %s): %w", r.packageRoot, err)
	}
	dataStreamManifests, err := packages.ReadAllDataStreamManifests(r.packageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading data stream manifests failed (path: %s): %w", r.packageRoot, err)
	}

	selectedSet := map[string]struct{}{}
	if selectedOnly {
		selectedDataStreams, err := r.selectedDataStreamsForRun()
		if err != nil {
			return nil, err
		}
		for _, dataStream := range selectedDataStreams {
			selectedSet[dataStream] = struct{}{}
		}
	}

	templateNames := make([]string, 0, len(dataStreamManifests))
	for _, dataStreamManifest := range dataStreamManifests {
		if selectedOnly && len(selectedSet) > 0 {
			if _, found := selectedSet[dataStreamManifest.Name]; !found {
				continue
			}
		}
		if dataStreamManifest.Type != "logs" {
			continue
		}

		dataset := dataStreamManifest.Dataset
		if dataset == "" {
			dataset = packageManifest.Name + "." + dataStreamManifest.Name
		}
		templateNames = append(templateNames, "logs-"+dataset+"@custom")
	}

	if selectedOnly && len(templateNames) == 0 {
		return []string{defaultLogsDBColumnarComponentTemplateName}, nil
	}
	return templateNames, nil
}

func (r *runner) selectedDataStreamsForRun() ([]string, error) {
	if r.runSetup || r.runTearDown || r.runTestsOnly {
		configFilePath := r.configFilePath
		if r.runTearDown || r.runTestsOnly {
			serviceState, err := readServiceStateData(r.serviceStateFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read service state: %w", err)
			}
			configFilePath = serviceState.ConfigFilePath
		}
		if configFilePath == "" {
			return nil, nil
		}
		dataStream := testrunner.ExtractDataStreamFromPath(configFilePath, r.packageRoot)
		if dataStream == "" {
			return nil, nil
		}
		return []string{dataStream}, nil
	}

	if len(r.dataStreams) > 0 {
		return r.dataStreams, nil
	}
	return nil, nil
}

func buildLogsDBColumnarTemplatePayload(currentTemplate json.RawMessage, exists bool, propertyOverrides map[string]map[string]any, dynamicTemplates []map[string]any) ([]byte, error) {
	template := map[string]any{}
	if exists {
		if err := json.Unmarshal(currentTemplate, &template); err != nil {
			return nil, fmt.Errorf("failed to decode existing component template: %w", err)
		}
	}

	templateSection, ok := template["template"].(map[string]any)
	if !ok {
		templateSection = map[string]any{}
		template["template"] = templateSection
	}
	settingsSection, ok := templateSection["settings"].(map[string]any)
	if !ok {
		settingsSection = map[string]any{}
		templateSection["settings"] = settingsSection
	}
	indexSection, ok := settingsSection["index"].(map[string]any)
	if !ok {
		indexSection = map[string]any{}
		settingsSection["index"] = indexSection
	}
	indexSection["mode"] = logsDBColumnarIndexMode

	if len(propertyOverrides) > 0 || len(dynamicTemplates) > 0 {
		mappingsSection, ok := templateSection["mappings"].(map[string]any)
		if !ok {
			mappingsSection = map[string]any{}
			templateSection["mappings"] = mappingsSection
		}

		if len(propertyOverrides) > 0 {
			propertiesSection, ok := mappingsSection["properties"].(map[string]any)
			if !ok {
				propertiesSection = map[string]any{}
				mappingsSection["properties"] = propertiesSection
			}
			mergeMappingProperties(propertiesSection, nestFieldMappingOverrides(propertyOverrides))
		}

		if len(dynamicTemplates) > 0 {
			mappingsSection["dynamic_templates"] = prependDynamicTemplates(mappingsSection["dynamic_templates"], dynamicTemplates)
		}
	}

	payload, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("failed to encode logsdb columnar component template payload: %w", err)
	}
	return payload, nil
}

func collectDocValuesDisabledFieldOverrides(componentTemplate json.RawMessage) map[string]map[string]any {
	var template struct {
		Template struct {
			Mappings struct {
				Properties map[string]any `json:"properties"`
			} `json:"mappings"`
		} `json:"template"`
	}
	if err := json.Unmarshal(componentTemplate, &template); err != nil {
		return nil
	}
	return collectDocValuesDisabledFields(template.Template.Mappings.Properties, "")
}

// stripSubobjectsFromComponentTemplate removes explicit subobjects mapping
// parameters. logsdb_columnar rejects them because the mode already disables
// subobjects. Returns the (possibly unchanged) template and whether anything
// was removed.
func stripSubobjectsFromComponentTemplate(componentTemplate json.RawMessage) ([]byte, bool, error) {
	template := map[string]any{}
	if err := json.Unmarshal(componentTemplate, &template); err != nil {
		return nil, false, fmt.Errorf("failed to decode component template: %w", err)
	}

	templateSection, ok := template["template"].(map[string]any)
	if !ok {
		return componentTemplate, false, nil
	}
	mappingsSection, ok := templateSection["mappings"].(map[string]any)
	if !ok {
		return componentTemplate, false, nil
	}

	if !stripSubobjectsFromMappings(mappingsSection) {
		return componentTemplate, false, nil
	}

	payload, err := json.Marshal(template)
	if err != nil {
		return nil, false, fmt.Errorf("failed to encode component template without subobjects: %w", err)
	}
	return payload, true, nil
}

// sanitizeComponentTemplateForPut removes GET-only system fields that
// Elasticsearch rejects on PutComponentTemplate.
func sanitizeComponentTemplateForPut(componentTemplate []byte) ([]byte, error) {
	template := map[string]any{}
	if err := json.Unmarshal(componentTemplate, &template); err != nil {
		return nil, fmt.Errorf("failed to decode component template: %w", err)
	}

	changed := false
	for _, key := range []string{"created_date_millis", "modified_date_millis", "created_date", "modified_date"} {
		if _, ok := template[key]; ok {
			delete(template, key)
			changed = true
		}
	}
	if !changed {
		return componentTemplate, nil
	}

	payload, err := json.Marshal(template)
	if err != nil {
		return nil, fmt.Errorf("failed to encode sanitized component template: %w", err)
	}
	return payload, nil
}

func stripSubobjectsFromMappings(mappings map[string]any) bool {
	changed := false
	if _, ok := mappings["subobjects"]; ok {
		delete(mappings, "subobjects")
		changed = true
	}
	if properties, ok := mappings["properties"].(map[string]any); ok {
		if stripSubobjectsFromProperties(properties) {
			changed = true
		}
	}
	return changed
}

func stripSubobjectsFromProperties(properties map[string]any) bool {
	changed := false
	for _, raw := range properties {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := prop["subobjects"]; ok {
			delete(prop, "subobjects")
			changed = true
		}
		if nested, ok := prop["properties"].(map[string]any); ok {
			if stripSubobjectsFromProperties(nested) {
				changed = true
			}
		}
	}
	return changed
}

func collectDocValuesDisabledFields(properties map[string]any, prefix string) map[string]map[string]any {
	if len(properties) == 0 {
		return nil
	}

	overrides := map[string]map[string]any{}
	for name, raw := range properties {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		if nested, ok := prop["properties"].(map[string]any); ok {
			for nestedPath, mapping := range collectDocValuesDisabledFields(nested, path) {
				overrides[nestedPath] = mapping
			}
			continue
		}

		docValues, hasDocValues := prop["doc_values"].(bool)
		if !hasDocValues || docValues {
			continue
		}

		override := maps.Clone(prop)
		override["doc_values"] = true
		overrides[path] = override
	}
	return overrides
}

func nestFieldMappingOverrides(flat map[string]map[string]any) map[string]any {
	root := map[string]any{}
	for path, mapping := range flat {
		parts := strings.Split(path, ".")
		current := root
		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = mapping
				break
			}
			child, ok := current[part].(map[string]any)
			if !ok {
				child = map[string]any{}
				current[part] = child
			}
			props, ok := child["properties"].(map[string]any)
			if !ok {
				props = map[string]any{}
				child["properties"] = props
			}
			current = props
		}
	}
	return root
}

func mergeMappingProperties(dst, src map[string]any) {
	for key, srcValue := range src {
		srcMap, srcIsMap := srcValue.(map[string]any)
		if !srcIsMap {
			dst[key] = srcValue
			continue
		}

		dstValue, exists := dst[key]
		if !exists {
			dst[key] = srcValue
			continue
		}
		dstMap, dstIsMap := dstValue.(map[string]any)
		if !dstIsMap {
			dst[key] = srcValue
			continue
		}

		srcProps, srcHasProps := srcMap["properties"].(map[string]any)
		if !srcHasProps {
			dst[key] = srcValue
			continue
		}

		dstProps, ok := dstMap["properties"].(map[string]any)
		if !ok {
			dstProps = map[string]any{}
			dstMap["properties"] = dstProps
		}
		mergeMappingProperties(dstProps, srcProps)
	}
}

func prependDynamicTemplates(existing any, templates []map[string]any) []any {
	result := make([]any, 0, len(templates)+8)
	for _, template := range templates {
		result = append(result, template)
	}
	switch current := existing.(type) {
	case []any:
		result = append(result, current...)
	case nil:
		// nothing
	default:
		// Unexpected shape from JSON decode; keep only our workarounds.
	}
	return result
}

func (r *runner) GetTests(ctx context.Context) ([]testrunner.Tester, error) {
	var folders []testrunner.TestFolder
	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed (path: %s): %w", r.packageRoot, err)
	}

	hasDataStreams, err := testrunner.PackageHasDataStreams(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if package has data streams: %w", err)
	}

	if r.runSetup || r.runTearDown || r.runTestsOnly {
		_, err := os.Stat(r.serviceStateFilePath)
		logger.Debugf("Service state data exists in %s: %v", r.serviceStateFilePath, !os.IsNotExist(err))
		if r.runSetup && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run --setup, required to tear down previous setup")
		}
		if r.runTestsOnly && os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run tests with --no-provision, setup first with --setup")
		}
		if r.runTearDown && os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run --tear-down, setup not found")
		}
	} else {
		if _, err = os.Stat(r.serviceStateFilePath); !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run tests, required to tear down previous state run (path: %s)", r.serviceStateFilePath)
		}
	}

	var serviceState ServiceState
	if r.runTearDown || r.runTestsOnly {
		serviceState, err = readServiceStateData(r.serviceStateFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read service state: %w", err)
		}
	}

	if hasDataStreams {
		var dataStreams []string
		if r.runSetup || r.runTearDown || r.runTestsOnly {
			configFilePath := r.configFilePath
			if r.runTearDown || r.runTestsOnly {
				configFilePath = serviceState.ConfigFilePath
			}
			dataStream := testrunner.ExtractDataStreamFromPath(configFilePath, r.packageRoot)
			dataStreams = append(dataStreams, dataStream)
		} else if len(r.dataStreams) > 0 {
			dataStreams = r.dataStreams
		}

		folders, err = testrunner.FindTestFolders(r.packageRoot, dataStreams, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}

		if r.failOnMissingTests && len(folders) == 0 {
			if len(dataStreams) > 0 {
				return nil, fmt.Errorf("no %s tests found for %s data stream(s)", r.Type(), strings.Join(dataStreams, ","))
			}
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	} else {
		folders, err = testrunner.FindTestFolders(r.packageRoot, nil, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}
		if r.failOnMissingTests && len(folders) == 0 {
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	}

	if r.runSetup || r.runTearDown || r.runTestsOnly {
		// variant flag is not checked here since there are packages that do not have variants
		if len(folders) != 1 {
			return nil, fmt.Errorf("wrong number of test folders (expected 1): %d", len(folders))
		}
	}

	var testers []testrunner.Tester
	for _, t := range folders {
		folderTesters, err := r.createTestersForFolder(t, serviceState)
		if err != nil {
			return nil, err
		}
		testers = append(testers, folderTesters...)
	}
	return testers, nil
}

func (r *runner) createTestersForFolder(testFolder testrunner.TestFolder, serviceState ServiceState) ([]testrunner.Tester, error) {
	var variants []string
	var cfgFiles []string
	var err error

	if r.runTestsOnly || r.runTearDown {
		variants = []string{serviceState.VariantName}
		cfgFiles = []string{filepath.Base(serviceState.ConfigFilePath)}
	} else {
		variants, err = r.getAllVariants(testFolder)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve variants from %s: %w", testFolder.Path, err)
		}

		cfgFiles, err = r.getAllConfigFiles(testFolder)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve config files from %s: %w", testFolder.Path, err)
		}
	}

	testers := make([]testrunner.Tester, 0, len(variants)*len(cfgFiles))
	for _, variant := range variants {
		for _, config := range cfgFiles {
			logger.Debugf("System runner: data stream %q config file %q variant %q", testFolder.DataStream, config, variant)
			tester, err := NewSystemTester(SystemTesterOptions{
				Profile:              r.profile,
				PackageRoot:          r.packageRoot,
				KibanaClient:         r.kibanaClient,
				API:                  r.esAPI,
				ESClient:             r.esClient,
				SchemaURLs:           r.schemaURLs,
				TestFolder:           testFolder,
				ServiceVariant:       variant,
				GenerateTestResult:   r.generateTestResult,
				DeferCleanup:         r.deferCleanup,
				RunSetup:             r.runSetup,
				RunTestsOnly:         r.runTestsOnly,
				RunTearDown:          r.runTearDown,
				ConfigFileName:       config,
				GlobalTestConfig:     r.globalTestConfig,
				WithCoverage:         r.withCoverage,
				CoverageType:         r.coverageType,
				OverrideAgentVersion: r.overrideAgentVersion,
			})
			if err != nil {
				return nil, fmt.Errorf(
					"failed to create system runner for sdata stream %q variant %q config file %q: %w",
					testFolder.DataStream, variant, config, err)
			}
			testers = append(testers, tester)
		}
	}

	return testers, nil
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *runner) resources(opts resourcesOptions) resources.Resources {
	return resources.Resources{
		&resources.FleetPackage{
			PackageRoot:            r.packageRoot,
			Absent:                 !opts.installedPackage,
			Force:                  opts.installedPackage, // Force re-installation, in case there are code changes in the same package version.
			RepositoryRoot:         r.repositoryRoot,
			SchemaURLs:             r.schemaURLs,
			RequiredInputsResolver: r.requiredInputsResolver,
		},
	}
}

func (r *runner) selectVariants(variantsFile *servicedeployer.VariantsFile) []string {
	if variantsFile == nil || variantsFile.Variants == nil {
		return []string{""} // empty variants file switches to no-variant mode
	}

	var variantNames []string
	for k := range variantsFile.Variants {
		if r.serviceVariant != "" && r.serviceVariant != k {
			continue
		}
		variantNames = append(variantNames, k)
	}
	return variantNames
}

func (r *runner) getAllVariants(folder testrunner.TestFolder) ([]string, error) {
	var variants []string
	dataStreamRoot, found, err := packages.FindDataStreamRootForPath(folder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data stream root failed: %w", err)
	}
	if found {
		logger.Debugf("Running system tests for data stream %q", folder.DataStream)
	} else {
		logger.Debug("Running system tests for package")
	}
	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		PackageRoot:    r.packageRoot,
		DataStreamRoot: dataStreamRoot,
		DevDeployDir:   DevDeployDir,
	})
	switch {
	case errors.Is(err, os.ErrNotExist):
		variants = r.selectVariants(nil)
	case err != nil:
		return nil, fmt.Errorf("failed fo find service deploy path: %w", err)
	default:
		variantsFile, err := servicedeployer.ReadVariantsFile(devDeployPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("can't read service variant: %w", err)
		}
		variants = r.selectVariants(variantsFile)
	}
	if r.serviceVariant != "" && len(variants) == 0 {
		return nil, fmt.Errorf("not found variant definition %q", r.serviceVariant)
	}

	if r.runSetup {
		// variant information in runTestOnly or runTearDown modes is retrieved from serviceOptions (file in setup dir)
		if len(variants) > 1 {
			return nil, fmt.Errorf("a variant must be selected or trigger the test in no-variant mode (available variants: %s)", strings.Join(variants, ", "))
		}
		if len(variants) == 1 && variants[0] == "" {
			logger.Debug("No variant mode")
		}
	}

	return variants, nil
}

func (r *runner) getAllConfigFiles(folder testrunner.TestFolder) ([]string, error) {
	var cfgFiles []string
	var err error
	if r.configFilePath != "" {
		allCfgFiles, err := listConfigFiles(filepath.Dir(r.configFilePath))
		if err != nil {
			return nil, fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
		baseFile := filepath.Base(r.configFilePath)
		for _, cfg := range allCfgFiles {
			if cfg == baseFile {
				cfgFiles = append(cfgFiles, baseFile)
			}
		}
	} else {
		cfgFiles, err = listConfigFiles(folder.Path)
		if err != nil {
			return nil, fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
	}
	return cfgFiles, nil
}
