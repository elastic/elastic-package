// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
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

	defaultExternal = "ecs"
)

// Validator is responsible for fields validation.
type Validator struct {
	// Schema contains definition records.
	Schema []FieldDefinition

	// SpecVersion contains the version of the spec used by the package.
	specVersion semver.Version

	// expectedDataset contains the value expected for dataset fields.
	expectedDataset string

	defaultNumericConversion bool
	numericKeywordFields     map[string]struct{}

	disabledDependencyManagement bool

	enabledAllowedIPCheck bool
	allowedCIDRs          []*net.IPNet

	enabledImportAllECSSchema bool

	disabledNormalization bool
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

// WithExpectedDataset configures the validator to check if the dataset fields have the expected values.
func WithExpectedDataset(dataset string) ValidatorOption {
	return func(v *Validator) error {
		v.expectedDataset = dataset
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
		// As every command starts with approximating where is the package root, it isn't required to return an error in case the root is missing.
		// This is also useful for testing purposes, where we don't have a real package, but just "fields" directory. The package root is always absent.
		if !found {
			logger.Debug("Package root not found, dependency management will be disabled.")
			v.disabledDependencyManagement = true
		} else {
			fdm, v.Schema, err = initDependencyManagement(packageRoot, v.specVersion, v.enabledImportAllECSSchema)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize dependency management: %w", err)
			}
		}
	}

	fields, err := loadFieldsFromDir(fieldsDir, fdm)
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

	var schema []FieldDefinition
	if buildManifest.ImportMappings() && !specVersion.LessThan(semver2_3_0) && importECSSchema {
		ecsSchema, err := fdm.ImportAllFields(defaultExternal)
		if err != nil {
			return nil, nil, err
		}
		schema = ecsSchema
	}

	return fdm, schema, nil
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

func loadFieldsFromDir(fieldsDir string, fdm *DependencyManager) ([]FieldDefinition, error) {
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
			body, err = injectFields(body, fdm)
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

func injectFields(d []byte, dm *DependencyManager) ([]byte, error) {
	var fields []common.MapStr
	err := yaml.Unmarshal(d, &fields)
	if err != nil {
		return nil, fmt.Errorf("parsing fields failed: %w", err)
	}

	fields, _, err = dm.InjectFields(fields)
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
	if !v.specVersion.LessThan(semver2_0_0) && v.expectedDataset != "" {
		for _, datasetField := range datasetFieldNames {
			value, err := body.GetValue(datasetField)
			if err == common.ErrKeyNotFound {
				continue
			}

			str, ok := valueToString(value, v.disabledNormalization)
			if !ok || str != v.expectedDataset {
				err := fmt.Errorf("field %q should have value %q, it has \"%v\"",
					datasetField, v.expectedDataset, value)
				errs = append(errs, err)
			}
		}
	}
	return errs
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
	_, isNumber := val.(float64)
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
	case "group":
		switch val.(type) {
		case map[string]interface{}:
			// TODO: This is probably an element from an array of objects,
			// even if not recommended, it should be validated.
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
	if e := definition.ExpectedValues; len(e) > 0 && !common.StringSliceContains(e, value) {
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
		if !common.StringSliceContains(expected, eventType) {
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
