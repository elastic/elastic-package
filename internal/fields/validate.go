// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/cbroglie/mustache"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

var (
	semver2_0_0 = semver.MustParse("2.0.0")
	semver2_3_0 = semver.MustParse("2.3.0")
	semver3_0_1 = semver.MustParse("3.0.1")

	// List of stack releases that do not
	// include ECS mappings (all versions before 8.13.0).
	stackVersionsWithoutECS = []*semver.Version{
		semver.MustParse("8.12.2"),
		semver.MustParse("8.12.1"),
		semver.MustParse("8.12.0"),
		semver.MustParse("8.11.4"),
		semver.MustParse("8.11.3"),
		semver.MustParse("8.11.2"),
		semver.MustParse("8.11.1"),
		semver.MustParse("8.11.0"),
		semver.MustParse("8.10.4"),
		semver.MustParse("8.10.3"),
		semver.MustParse("8.10.2"),
		semver.MustParse("8.10.1"),
		semver.MustParse("8.10.0"),
		semver.MustParse("8.9.2"),
		semver.MustParse("8.9.1"),
		semver.MustParse("8.9.0"),
		semver.MustParse("8.8.2"),
		semver.MustParse("8.8.1"),
		semver.MustParse("8.8.0"),
		semver.MustParse("8.7.1"),
		semver.MustParse("8.7.0"),
		semver.MustParse("8.6.2"),
		semver.MustParse("8.6.1"),
		semver.MustParse("8.6.0"),
		semver.MustParse("8.5.3"),
		semver.MustParse("8.5.2"),
		semver.MustParse("8.5.1"),
		semver.MustParse("8.5.0"),
		semver.MustParse("8.4.3"),
		semver.MustParse("8.4.2"),
		semver.MustParse("8.4.1"),
		semver.MustParse("8.4.0"),
		semver.MustParse("8.3.3"),
		semver.MustParse("8.3.2"),
		semver.MustParse("8.3.1"),
		semver.MustParse("8.3.0"),
		semver.MustParse("8.2.3"),
		semver.MustParse("8.2.2"),
		semver.MustParse("8.2.1"),
		semver.MustParse("8.2.0"),
		semver.MustParse("8.1.3"),
		semver.MustParse("8.1.2"),
		semver.MustParse("8.1.1"),
		semver.MustParse("8.1.0"),
		semver.MustParse("8.0.1"),
		semver.MustParse("8.0.0"),
		semver.MustParse("7.17.24"),
		semver.MustParse("7.17.23"),
		semver.MustParse("7.17.22"),
		semver.MustParse("7.17.21"),
		semver.MustParse("7.17.20"),
		semver.MustParse("7.17.19"),
		semver.MustParse("7.17.18"),
		semver.MustParse("7.17.17"),
		semver.MustParse("7.17.16"),
		semver.MustParse("7.17.15"),
		semver.MustParse("7.17.14"),
		semver.MustParse("7.17.13"),
		semver.MustParse("7.17.12"),
		semver.MustParse("7.17.11"),
		semver.MustParse("7.17.10"),
		semver.MustParse("7.17.9"),
		semver.MustParse("7.17.8"),
		semver.MustParse("7.17.7"),
		semver.MustParse("7.17.6"),
		semver.MustParse("7.17.5"),
		semver.MustParse("7.17.4"),
		semver.MustParse("7.17.3"),
		semver.MustParse("7.17.2"),
		semver.MustParse("7.17.1"),
		semver.MustParse("7.17.0"),
		semver.MustParse("7.16.3"),
		semver.MustParse("7.16.2"),
		semver.MustParse("7.16.1"),
		semver.MustParse("7.16.0"),
		semver.MustParse("7.15.2"),
		semver.MustParse("7.15.1"),
		semver.MustParse("7.15.0"),
		semver.MustParse("7.14.2"),
		semver.MustParse("7.14.1"),
		semver.MustParse("7.14.0"), // First version of Fleet in GA; there are no packages older than this version.
	}

	defaultExternal = "ecs"
)

// Validator is responsible for fields validation.
type Validator struct {
	// Schema contains definition records.
	Schema []FieldDefinition

	// SpecVersion contains the version of the spec used by the package.
	specVersion semver.Version

	// expectedDatasets contains the value expected for dataset fields.
	expectedDatasets []string

	defaultNumericConversion bool
	numericKeywordFields     map[string]struct{}

	disabledDependencyManagement bool

	enabledAllowedIPCheck bool
	allowedCIDRs          []*net.IPNet

	enabledImportAllECSSchema bool

	disabledNormalization bool

	injectFieldsOptions InjectFieldsOptions
}

// ValidatorOption represents an optional flag that can be passed to  CreateValidatorForDirectory.
type ValidatorOption func(*Validator) error

// WithSpecVersion enables validation dependant of the spec version used by the package.
func WithSpecVersion(version string) ValidatorOption {
	return func(v *Validator) error {
		sv, err := semver.NewVersion(version)
		if err != nil {
			return fmt.Errorf("invalid version %q: %v", version, err)
		}
		v.specVersion = *sv
		return nil
	}
}

// WithDefaultNumericConversion configures the validator to accept defined keyword (or constant_keyword) fields as numeric-type.
func WithDefaultNumericConversion() ValidatorOption {
	return func(v *Validator) error {
		v.defaultNumericConversion = true
		return nil
	}
}

// WithNumericKeywordFields configures the validator to accept specific fields to have numeric-type
// while defined as keyword or constant_keyword.
func WithNumericKeywordFields(fields []string) ValidatorOption {
	return func(v *Validator) error {
		v.numericKeywordFields = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			v.numericKeywordFields[field] = struct{}{}
		}
		return nil
	}
}

// WithDisabledDependencyManagement configures the validator to ignore external fields and won't follow dependencies.
func WithDisabledDependencyManagement() ValidatorOption {
	return func(v *Validator) error {
		v.disabledDependencyManagement = true
		return nil
	}
}

// WithEnabledAllowedIPCheck configures the validator to perform check on the IP values against an allowed list.
func WithEnabledAllowedIPCheck() ValidatorOption {
	return func(v *Validator) error {
		v.enabledAllowedIPCheck = true
		return nil
	}
}

// WithExpectedDatasets configures the validator to check if the dataset field value matches one of the expected values.
func WithExpectedDatasets(datasets []string) ValidatorOption {
	return func(v *Validator) error {
		v.expectedDatasets = datasets
		return nil
	}
}

// WithEnabledImportAllECSSchema configures the validator to check or not the fields with the complete ECS schema.
func WithEnabledImportAllECSSChema(importSchema bool) ValidatorOption {
	return func(v *Validator) error {
		v.enabledImportAllECSSchema = importSchema
		return nil
	}
}

// WithDisableNormalization configures the validator to disable normalization.
func WithDisableNormalization(disabledNormalization bool) ValidatorOption {
	return func(v *Validator) error {
		v.disabledNormalization = disabledNormalization
		return nil
	}
}

// WithInjectFieldsOptions configures fields injection.
func WithInjectFieldsOptions(options InjectFieldsOptions) ValidatorOption {
	return func(v *Validator) error {
		v.injectFieldsOptions = options
		return nil
	}
}

type packageRootFinder interface {
	FindPackageRoot() (string, bool, error)
}

type packageRoot struct{}

func (p packageRoot) FindPackageRoot() (string, bool, error) {
	return packages.FindPackageRoot()
}

// CreateValidatorForDirectory function creates a validator for the directory.
func CreateValidatorForDirectory(fieldsParentDir string, opts ...ValidatorOption) (v *Validator, err error) {
	p := packageRoot{}
	return createValidatorForDirectoryAndPackageRoot(fieldsParentDir, p, opts...)
}

func createValidatorForDirectoryAndPackageRoot(fieldsParentDir string, finder packageRootFinder, opts ...ValidatorOption) (v *Validator, err error) {
	v = new(Validator)
	// In validator, inject fields with settings used for validation, such as `allowed_values`.
	v.injectFieldsOptions.IncludeValidationSettings = true
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	v.allowedCIDRs = initializeAllowedCIDRsList()

	fieldsDir := filepath.Join(fieldsParentDir, "fields")

	var fdm *DependencyManager
	if !v.disabledDependencyManagement {
		packageRoot, found, err := finder.FindPackageRoot()
		if err != nil {
			return nil, fmt.Errorf("can't find package root: %w", err)
		}
		if !found {
			return nil, errors.New("package root not found and dependency management is enabled")
		}
		fdm, v.Schema, err = initDependencyManagement(packageRoot, v.specVersion, v.enabledImportAllECSSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize dependency management: %w", err)
		}
	}

	fields, err := loadFieldsFromDir(fieldsDir, fdm, v.injectFieldsOptions)
	if err != nil {
		return nil, fmt.Errorf("can't load fields from directory (path: %s): %w", fieldsDir, err)
	}

	v.Schema = append(fields, v.Schema...)
	return v, nil
}

func initDependencyManagement(packageRoot string, specVersion semver.Version, importECSSchema bool) (*DependencyManager, []FieldDefinition, error) {
	buildManifest, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("can't read build manifest: %w", err)
	}
	if !ok {
		// There is no build manifest, nothing to do.
		return nil, nil, nil
	}

	fdm, err := CreateFieldDependencyManager(buildManifest.Dependencies)
	if err != nil {
		return nil, nil, fmt.Errorf("can't create field dependency manager: %w", err)
	}

	// Check if the package embeds ECS mappings
	packageEmbedsEcsMappings := buildManifest.ImportMappings() && !specVersion.LessThan(semver2_3_0)
	if !packageEmbedsEcsMappings {
		logger.Debugf("Package does not embed ECS mappings")
	}

	// Check if all stack versions support ECS mappings
	stackSupportsEcsMapping, err := supportsECSMappings(packageRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("can't check if stack version includes ECS mappings: %w", err)
	}

	// If the package embeds ECS mappings, or the stack version includes ECS mappings, then
	// we should import the ECS schema to validate the package fields against it.
	var schema []FieldDefinition
	if (packageEmbedsEcsMappings || stackSupportsEcsMapping) && importECSSchema {
		// Import all fields from external schema (most likely ECS) to
		// validate the package fields against it.
		ecsSchema, err := fdm.ImportAllFields(defaultExternal)
		if err != nil {
			return nil, nil, err
		}
		logger.Debug("Imported ECS fields definition from external schema for validation")
		schema = ecsSchema
	}

	return fdm, schema, nil
}

// supportsECSMappings check if all the versions of the stack the package can run on support ECS mappings.
func supportsECSMappings(packageRoot string) (bool, error) {
	packageManifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return false, fmt.Errorf("can't read package manifest: %w", err)
	}

	if len(packageManifest.Conditions.Kibana.Version) == 0 {
		logger.Debugf("No Kibana version constraint found in package manifest; assuming it does not support ECS mappings.")
		return false, nil
	}

	kibanaConstraints, err := semver.NewConstraint(packageManifest.Conditions.Kibana.Version)
	if err != nil {
		return false, fmt.Errorf("invalid constraint for Kibana: %w", err)
	}

	return allVersionsIncludeECS(kibanaConstraints), nil
}

// allVersionsIncludeECS Check if all the stack versions in the constraints include ECS mappings. Only the stack
// versions 8.13.0 and above include ECS mappings.
//
// Returns true if all the stack versions in the constraints include ECS mappings, otherwise returns false.
func allVersionsIncludeECS(kibanaConstraints *semver.Constraints) bool {
	// Looking for a version that satisfies the package constraints.
	for _, v := range stackVersionsWithoutECS {
		if kibanaConstraints.Check(v) {
			// Found a version that satisfies the constraints,
			// so at least this version does not include
			// ECS mappings.
			return false
		}
	}

	// If no version satisfies the constraints, then all versions
	// include ECS mappings.
	return true

	// This check works under the assumption the constraints are not limited
	// upwards.
	//
	// For example, if the constraint is `>= 8.12.0` and the stack version is
	// `8.12.999`, the constraint will be satisfied.
	//
	// However, if the constraint is `>= 8.0.0, < 8.10.0` the check will not
	// return the right result.
	//
	// To support this, we would need to check the constraint against a larger
	// set of versions, and check if the constraint is satisfied for all
	// of them, like in the commented out example above.
	//
	// lastStackVersionWithoutEcsMappings := semver.MustParse("8.12.999")
	// return !kibanaConstraints.Check(lastStackVersionWithoutEcsMappings)
}

//go:embed _static/allowed_geo_ips.txt
var allowedGeoIPs string

func initializeAllowedCIDRsList() (cidrs []*net.IPNet) {
	s := bufio.NewScanner(strings.NewReader(allowedGeoIPs))
	for s.Scan() {
		_, cidr, err := net.ParseCIDR(s.Text())
		if err != nil {
			panic("invalid ip in _static/allowed_geo_ips.txt: " + s.Text())
		}
		cidrs = append(cidrs, cidr)
	}

	return cidrs
}

func loadFieldsFromDir(fieldsDir string, fdm *DependencyManager, injectOptions InjectFieldsOptions) ([]FieldDefinition, error) {
	files, err := filepath.Glob(filepath.Join(fieldsDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("reading directory with fields failed (path: %s): %w", fieldsDir, err)
	}

	var fields []FieldDefinition
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading fields file failed: %w", err)
		}

		if fdm != nil {
			body, err = injectFields(body, fdm, injectOptions)
			if err != nil {
				return nil, fmt.Errorf("loading external fields failed: %w", err)
			}
		}

		var u []FieldDefinition
		err = yaml.Unmarshal(body, &u)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling field body failed: %w", err)
		}
		fields = append(fields, u...)
	}
	return fields, nil
}

func injectFields(d []byte, dm *DependencyManager, options InjectFieldsOptions) ([]byte, error) {
	var fields []common.MapStr
	err := yaml.Unmarshal(d, &fields)
	if err != nil {
		return nil, fmt.Errorf("parsing fields failed: %w", err)
	}

	fields, _, err = dm.injectFieldsWithOptions(fields, options)
	if err != nil {
		return nil, fmt.Errorf("injecting fields failed: %w", err)
	}

	return yaml.Marshal(fields)
}

// ValidateDocumentBody validates the provided document body.
func (v *Validator) ValidateDocumentBody(body json.RawMessage) multierror.Error {
	var c common.MapStr
	err := json.Unmarshal(body, &c)
	if err != nil {
		var errs multierror.Error
		errs = append(errs, fmt.Errorf("unmarshalling document body failed: %w", err))
		return errs
	}

	return v.ValidateDocumentMap(c)
}

// ValidateDocumentMap validates the provided document as common.MapStr.
func (v *Validator) ValidateDocumentMap(body common.MapStr) multierror.Error {
	errs := v.validateDocumentValues(body)
	errs = append(errs, v.validateMapElement("", body, body)...)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

var datasetFieldNames = []string{
	"event.dataset",
	"data_stream.dataset",
}

func (v *Validator) validateDocumentValues(body common.MapStr) multierror.Error {
	var errs multierror.Error
	if !v.specVersion.LessThan(semver2_0_0) && v.expectedDatasets != nil {
		for _, datasetField := range datasetFieldNames {
			value, err := body.GetValue(datasetField)
			if errors.Is(err, common.ErrKeyNotFound) {
				continue
			}

			// Why do we render the expected datasets here?
			// Because the expected datasets can contain
			// mustache templates, and not just static
			// strings.
			//
			// For example, the expected datasets for the
			// Kubernetes container logs dataset can be:
			//
			//   - "{{kubernetes.labels.elastic_co/dataset}}"
			//
			var renderedExpectedDatasets []string
			for _, dataset := range v.expectedDatasets {
				renderedDataset, err := mustache.Render(dataset, body)
				if err != nil {
					err := fmt.Errorf("can't render expected dataset %q: %w", dataset, err)
					errs = append(errs, err)
					return errs
				}
				renderedExpectedDatasets = append(renderedExpectedDatasets, renderedDataset)
			}

			str, ok := valueToString(value, v.disabledNormalization)
			exists := stringInArray(str, renderedExpectedDatasets)
			if !ok || !exists {
				err := fmt.Errorf("field %q should have value in %q, it has \"%v\"",
					datasetField, v.expectedDatasets, value)
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func stringInArray(target string, arr []string) bool {
	// Check if target is part of the array
	found := false
	for _, item := range arr {
		if item == target {
			found = true
			break
		}
	}
	return found
}

func valueToString(value any, disabledNormalization bool) (string, bool) {
	if disabledNormalization {
		// when synthetics mode is enabled, each field present in the document is an array
		// so this check needs to retrieve the first element of the array
		vals, err := common.ToStringSlice(value)
		if err != nil || len(vals) != 1 {
			return "", false
		}
		return vals[0], true
	}
	str, ok := value.(string)
	return str, ok
}

func (v *Validator) validateMapElement(root string, elem common.MapStr, doc common.MapStr) multierror.Error {
	var errs multierror.Error
	for name, val := range elem {
		key := strings.TrimLeft(root+"."+name, ".")

		switch val := val.(type) {
		case []map[string]interface{}:
			for _, m := range val {
				err := v.validateMapElement(key, m, doc)
				if err != nil {
					errs = append(errs, err...)
				}
			}
		case map[string]interface{}:
			if isFieldTypeFlattened(key, v.Schema) {
				// Do not traverse into objects with flattened data types
				// because the entire object is mapped as a single field.
				continue
			}
			err := v.validateMapElement(key, val, doc)
			if err != nil {
				errs = append(errs, err...)
			}
		default:
			err := v.validateScalarElement(key, val, doc)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (v *Validator) validateScalarElement(key string, val interface{}, doc common.MapStr) error {
	if key == "" {
		return nil // root key is always valid
	}

	definition := FindElementDefinition(key, v.Schema)
	if definition == nil && skipValidationForField(key) {
		return nil // generic field, let's skip validation for now
	}

	if definition == nil {
		return fmt.Errorf(`field "%s" is undefined`, key)
	}

	// Convert numeric keyword fields to string for validation.
	_, found := v.numericKeywordFields[key]
	if (found || v.defaultNumericConversion) && isNumericKeyword(*definition, val) {
		val = fmt.Sprintf("%q", val)
	}

	if !v.disabledNormalization {
		err := v.validateExpectedNormalization(*definition, val)
		if err != nil {
			return fmt.Errorf("field %q is not normalized as expected: %w", key, err)
		}
	}

	err := v.parseElementValue(key, *definition, val, doc)
	if err != nil {
		return fmt.Errorf("parsing field value failed: %w", err)
	}
	return nil
}

func (v *Validator) SanitizeSyntheticSourceDocs(docs []common.MapStr) ([]common.MapStr, error) {
	var newDocs []common.MapStr
	for _, doc := range docs {
		for key, contents := range doc {
			shouldBeArray := false
			definition := FindElementDefinition(key, v.Schema)
			if definition != nil {
				shouldBeArray = v.shouldValueBeArray(definition)
			}

			// if it needs to be normalized, the field is kept as it is
			if shouldBeArray {
				continue
			}
			// in case it is not specified any normalization and that field is an array of
			// just one element, the field is going to be updated to remove the array and keep
			// that element as a value.
			vals, ok := contents.([]interface{})
			if !ok {
				continue
			}
			if len(vals) == 1 {
				_, err := doc.Put(key, vals[0])
				if err != nil {
					return nil, fmt.Errorf("key %s could not be updated: %w", key, err)
				}
			}
		}
		expandedDoc, err := createDocExpandingObjects(doc)
		if err != nil {
			return nil, fmt.Errorf("failure while expanding objects from doc: %w", err)
		}

		newDocs = append(newDocs, expandedDoc)
	}
	return newDocs, nil
}

func (v *Validator) shouldValueBeArray(definition *FieldDefinition) bool {
	// normalization should just be checked if synthetic source is enabled and the
	// spec version of this package is >= 2.0.0
	if v.disabledNormalization && !v.specVersion.LessThan(semver2_0_0) {
		for _, normalize := range definition.Normalize {
			switch normalize {
			case "array":
				return true
			}
		}
	}
	return false
}

func createDocExpandingObjects(doc common.MapStr) (common.MapStr, error) {
	keys := make([]string, 0)
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	newDoc := make(common.MapStr)
	for _, k := range keys {
		value, err := doc.GetValue(k)
		if err != nil {
			return nil, fmt.Errorf("not found key %s: %w", k, err)
		}

		_, err = newDoc.Put(k, value)
		if err == nil {
			continue
		}

		// Possible errors found but not limited to those
		// - expected map but type is string
		// - expected map but type is []interface{}
		if strings.HasPrefix(err.Error(), "expected map but type is") {
			logger.Debugf("not able to add key %s, is this a multifield?: %s", k, err)
			continue
		}
		return nil, fmt.Errorf("not added key %s with value %s: %w", k, value, err)
	}
	return newDoc, nil
}
func isNumericKeyword(definition FieldDefinition, val interface{}) bool {
	var isNumber bool
	switch val := val.(type) {
	case bool, []bool, float64, []float64:
		isNumber = true
	case []interface{}:
		isNumber = true
	loop:
		for _, v := range val {
			switch v.(type) {
			case bool, float64:
			default:
				isNumber = false
				break loop
			}
		}
	}
	return isNumber && (definition.Type == "keyword" || definition.Type == "constant_keyword")
}

// skipValidationForField skips field validation (field presence) of special fields. The special fields are present
// in every (most?) documents collected by Elastic Agent, but aren't defined in any integration in `fields.yml` files.
// FIXME https://github.com/elastic/elastic-package/issues/147
func skipValidationForField(key string) bool {
	return isFieldFamilyMatching("agent", key) ||
		isFieldFamilyMatching("elastic_agent", key) ||
		isFieldFamilyMatching("cloud", key) || // too many common fields
		isFieldFamilyMatching("event", key) || // too many common fields
		isFieldFamilyMatching("host", key) || // too many common fields
		isFieldFamilyMatching("metricset", key) || // field is deprecated
		isFieldFamilyMatching("event.module", key) // field is deprecated
}

func isFieldFamilyMatching(family, key string) bool {
	return key == family || strings.HasPrefix(key, family+".")
}

func isFieldTypeFlattened(key string, fieldDefinitions []FieldDefinition) bool {
	definition := FindElementDefinition(key, fieldDefinitions)
	return definition != nil && definition.Type == "flattened"
}

func findElementDefinitionForRoot(root, searchedKey string, FieldDefinitions []FieldDefinition) *FieldDefinition {
	for _, def := range FieldDefinitions {
		key := strings.TrimLeft(root+"."+def.Name, ".")
		if compareKeys(key, def, searchedKey) {
			return &def
		}

		fd := findElementDefinitionForRoot(key, searchedKey, def.Fields)
		if fd != nil {
			return fd
		}

		fd = findElementDefinitionForRoot(key, searchedKey, def.MultiFields)
		if fd != nil {
			return fd
		}
	}
	return nil
}

// FindElementDefinition is a helper function used to find the fields definition in the schema.
func FindElementDefinition(searchedKey string, fieldDefinitions []FieldDefinition) *FieldDefinition {
	return findElementDefinitionForRoot("", searchedKey, fieldDefinitions)
}

// compareKeys checks if `searchedKey` matches with the given `key`. `key` can contain
// wildcards (`*`), that match any sequence of characters in `searchedKey` different to dots.
func compareKeys(key string, def FieldDefinition, searchedKey string) bool {
	// Loop over every byte in `key` to find if there is a matching byte in `searchedKey`.
	var j int
	for _, k := range []byte(key) {
		if j >= len(searchedKey) {
			// End of searched key reached before maching all characters in the key.
			return false
		}
		switch k {
		case searchedKey[j]:
			// Match, continue.
			j++
		case '*':
			// Wildcard, match everything till next dot.
			switch idx := strings.IndexByte(searchedKey[j:], '.'); idx {
			default:
				// Jump till next dot.
				j += idx
			case -1:
				// No dots, wildcard matches with the rest of the searched key.
				j = len(searchedKey)
			case 0:
				// Empty name on wildcard, this is not permitted (e.g. `example..foo`).
				return false
			}
		default:
			// No match.
			return false
		}
	}
	// If everything matched, searched key has been found.
	if len(searchedKey) == j {
		return true
	}

	// Workaround for potential subfields of certain types as geo_point or histogram.
	if len(searchedKey) > j {
		extraPart := searchedKey[j:]
		if validSubField(def, extraPart) {
			return true
		}
	}

	return false
}

func (v *Validator) validateExpectedNormalization(definition FieldDefinition, val interface{}) error {
	// Validate expected normalization starting with packages following spec v2 format.
	if v.specVersion.LessThan(semver2_0_0) {
		return nil
	}
	for _, normalize := range definition.Normalize {
		switch normalize {
		case "array":
			if _, isArray := val.([]interface{}); val != nil && !isArray {
				return fmt.Errorf("expected array, found %q (%T)", val, val)
			}
		}
	}
	return nil
}

// validSubField checks if the extra part that didn't match with any field definition,
// matches with the possible sub field of complex fields like geo_point or histogram.
func validSubField(def FieldDefinition, extraPart string) bool {
	fieldType := def.Type
	if def.Type == "object" && def.ObjectType != "" {
		fieldType = def.ObjectType
	}

	subFields := []string{".lat", ".lon", ".values", ".counts"}
	perType := map[string][]string{
		"geo_point": subFields[0:2],
		"histogram": subFields[2:4],
	}

	allowed, found := perType[fieldType]
	if !found {
		if def.External != "" {
			// An unresolved external field could be anything.
			allowed = subFields
		} else {
			return false
		}
	}

	for _, a := range allowed {
		if a == extraPart {
			return true
		}
	}

	return false
}

// parseElementValue checks that the value stored in a field matches the field definition. For
// arrays it checks it for each Element.
func (v *Validator) parseElementValue(key string, definition FieldDefinition, val interface{}, doc common.MapStr) error {
	err := v.parseAllElementValues(key, definition, val, doc)
	if err != nil {
		return err
	}

	return forEachElementValue(key, definition, val, doc, v.parseSingleElementValue)
}

// parseAllElementValues performs validations that must be done for all elements at once in
// case that there are multiple values.
func (v *Validator) parseAllElementValues(key string, definition FieldDefinition, val interface{}, doc common.MapStr) error {
	switch definition.Type {
	case "constant_keyword", "keyword", "text":
		if !v.specVersion.LessThan(semver2_0_0) {
			strings, err := valueToStringsSlice(val)
			if err != nil {
				return fmt.Errorf("field %q value \"%v\" (%T): %w", key, val, val, err)
			}
			if err := ensureExpectedEventType(key, strings, definition, doc); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseSingeElementValue performs validations on individual values of each element.
func (v *Validator) parseSingleElementValue(key string, definition FieldDefinition, val interface{}, doc common.MapStr) error {
	invalidTypeError := func() error {
		return fmt.Errorf("field %q's Go type, %T, does not match the expected field type: %s (field value: %v)", key, val, definition.Type, val)
	}

	switch definition.Type {
	// Constant keywords can define a value in the definition, if they do, all
	// values stored in this field should be this one.
	// If a pattern is provided, it checks if the value matches.
	case "constant_keyword":
		valStr, valid := val.(string)
		if !valid {
			return invalidTypeError()
		}

		if err := ensureConstantKeywordValueMatches(key, valStr, definition.Value); err != nil {
			return err
		}
		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}
		if err := ensureAllowedValues(key, valStr, definition); err != nil {
			return err
		}
	// Normal text fields should be of type string.
	// If a pattern is provided, it checks if the value matches.
	case "keyword", "text":
		valStr, valid := val.(string)
		if !valid {
			return invalidTypeError()
		}

		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}
		if err := ensureAllowedValues(key, valStr, definition); err != nil {
			return err
		}
	// Dates are expected to be formatted as strings or as seconds or milliseconds
	// since epoch.
	// If it is a string and a pattern is provided, it checks if the value matches.
	case "date":
		switch val := val.(type) {
		case string:
			if err := ensurePatternMatches(key, val, definition.Pattern); err != nil {
				return err
			}
		case float64:
			// date as seconds or milliseconds since epoch
			if definition.Pattern != "" {
				return fmt.Errorf("numeric date in field %q, but pattern defined", key)
			}
		default:
			return invalidTypeError()
		}
	// IP values should be actual IPs, included in the ranges of IPs available
	// in the geoip test database.
	// If a pattern is provided, it checks if the value matches.
	case "ip":
		valStr, valid := val.(string)
		if !valid {
			return invalidTypeError()
		}

		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}

		if v.enabledAllowedIPCheck && !v.isAllowedIPValue(valStr) {
			return fmt.Errorf("the IP %q is not one of the allowed test IPs (see: https://github.com/elastic/elastic-package/blob/main/internal/fields/_static/allowed_geo_ips.txt)", valStr)
		}
	// Groups should only contain nested fields, not single values.
	case "group", "nested":
		switch val := val.(type) {
		case map[string]interface{}:
			// This is probably an element from an array of objects,
			// even if not recommended, it should be validated.
			if v.specVersion.LessThan(semver3_0_1) {
				break
			}
			errs := v.validateMapElement(key, common.MapStr(val), doc)
			if len(errs) == 0 {
				return nil
			}
			return errs
		case []interface{}:
			// This can be an array of array of objects. Elasticsearh will probably
			// flatten this. So even if this is quite unexpected, let's try to handle it.
			if v.specVersion.LessThan(semver3_0_1) {
				break
			}
			return forEachElementValue(key, definition, val, doc, v.parseSingleElementValue)
		case nil:
			// The document contains a null, let's consider this like an empty array.
			return nil
		default:
			return fmt.Errorf("field %q is a group of fields, it cannot store values", key)
		}
	// Numbers should have been parsed as float64, otherwise they are not numbers.
	case "float", "long", "double":
		if _, valid := val.(float64); !valid {
			return invalidTypeError()
		}
	// All other types are considered valid not blocking validation.
	default:
		return nil
	}

	return nil
}

// isAllowedIPValue checks if the provided IP is allowed for testing
// The set of allowed IPs are:
// - private IPs as described in RFC 1918 & RFC 4193
// - public IPs allowed by MaxMind for testing
// - 0.0.0.0 and 255.255.255.255 for IPv4
// - 0:0:0:0:0:0:0:0 and ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff for IPv6
func (v *Validator) isAllowedIPValue(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}

	for _, allowedCIDR := range v.allowedCIDRs {
		if allowedCIDR.Contains(ip) {
			return true
		}
	}

	if ip.IsUnspecified() ||
		ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.Equal(net.IPv4bcast) {
		return true
	}

	return false
}

// forEachElementValue visits a function for each element in the given value if
// it is an array. If it is not an array, it calls the function with it.
func forEachElementValue(key string, definition FieldDefinition, val interface{}, doc common.MapStr, fn func(string, FieldDefinition, interface{}, common.MapStr) error) error {
	arr, isArray := val.([]interface{})
	if !isArray {
		return fn(key, definition, val, doc)
	}
	for _, element := range arr {
		err := fn(key, definition, element, doc)
		if err != nil {
			return err
		}
	}
	return nil
}

// ensurePatternMatches validates the document's field value matches the field
// definitions regular expression pattern.
func ensurePatternMatches(key, value, pattern string) error {
	if pattern == "" {
		return nil
	}
	valid, err := regexp.MatchString(pattern, value)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}
	if !valid {
		return fmt.Errorf("field %q's value, %s, does not match the expected pattern: %s", key, value, pattern)
	}
	return nil
}

// ensureConstantKeywordValueMatches validates the document's field value
// matches the definition's constant_keyword value.
func ensureConstantKeywordValueMatches(key, value, constantKeywordValue string) error {
	if constantKeywordValue == "" {
		return nil
	}
	if value != constantKeywordValue {
		return fmt.Errorf("field %q's value %q does not match the declared constant_keyword value %q", key, value, constantKeywordValue)
	}
	return nil
}

// ensureAllowedValues validates that the document's field value
// is one of the allowed values.
func ensureAllowedValues(key, value string, definition FieldDefinition) error {
	if !definition.AllowedValues.IsAllowed(value) {
		return fmt.Errorf("field %q's value %q is not one of the allowed values (%s)", key, value, strings.Join(definition.AllowedValues.Values(), ", "))
	}
	if e := definition.ExpectedValues; len(e) > 0 && !slices.Contains(e, value) {
		return fmt.Errorf("field %q's value %q is not one of the expected values (%s)", key, value, strings.Join(e, ", "))
	}
	return nil
}

// ensureExpectedEventType validates that the document's `event.type` field is one of the expected
// one for the given value.
func ensureExpectedEventType(key string, values []string, definition FieldDefinition, doc common.MapStr) error {
	eventTypeVal, _ := doc.GetValue("event.type")
	eventTypes, err := valueToStringsSlice(eventTypeVal)
	if err != nil {
		return fmt.Errorf("field \"event.type\" value \"%v\" (%T): %w", eventTypeVal, eventTypeVal, err)
	}
	var expected []string
	for _, value := range values {
		expectedForValue := definition.AllowedValues.ExpectedEventTypes(value)
		expected = common.StringSlicesUnion(expected, expectedForValue)
	}
	if len(expected) == 0 {
		// No restrictions defined for this value, all good to go.
		return nil
	}
	for _, eventType := range eventTypes {
		if !slices.Contains(expected, eventType) {
			return fmt.Errorf("field \"event.type\" value %q is not one of the expected values (%s) for any of the values of %q (%s)", eventType, strings.Join(expected, ", "), key, strings.Join(values, ", "))
		}
	}

	return nil
}

func valueToStringsSlice(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return []string{v}, nil
	case []interface{}:
		var values []string
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("expected string or array of strings")
			}
			values = append(values, s)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected string or array of strings")
	}
}
