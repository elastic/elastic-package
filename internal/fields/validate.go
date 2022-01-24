// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

// Validator is responsible for fields validation.
type Validator struct {
	// Schema contains definition records.
	Schema []FieldDefinition
	// FieldDependencyManager resolves references to external fields
	FieldDependencyManager *DependencyManager

	defaultNumericConversion bool
	numericKeywordFields     map[string]struct{}

	disabledDependencyManagement bool

	enabledAllowedIPCheck bool
	allowedIPs            map[string]struct{}
}

// ValidatorOption represents an optional flag that can be passed to  CreateValidatorForDataStream.
type ValidatorOption func(*Validator) error

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

// CreateValidatorForDataStream function creates a validator for the data stream.
func CreateValidatorForDataStream(dataStreamRootPath string, opts ...ValidatorOption) (v *Validator, err error) {
	v = new(Validator)
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	v.allowedIPs = initializeAllowedIPsList()

	v.Schema, err = loadFieldsForDataStream(dataStreamRootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "can't load fields for data stream (path: %s)", dataStreamRootPath)
	}

	if v.disabledDependencyManagement {
		return v, nil
	}

	packageRoot := filepath.Dir(filepath.Dir(dataStreamRootPath))
	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return nil, errors.Wrap(err, "can't read build manifest")
	}
	if !ok {
		v.disabledDependencyManagement = true
		return v, nil
	}

	fdm, err := CreateFieldDependencyManager(bm.Dependencies)
	if err != nil {
		return nil, errors.Wrap(err, "can't create field dependency manager")
	}
	v.FieldDependencyManager = fdm
	return v, nil
}

//go:embed _static/allowed_geo_ips.txt
var allowedGeoIPs string

func initializeAllowedIPsList() map[string]struct{} {
	m := map[string]struct{}{
		"0.0.0.0": {}, "255.255.255.255": {},
		"0:0:0:0:0:0:0:0": {}, "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff": {}, "::": {},
	}
	for _, ip := range strings.Split(allowedGeoIPs, "\n") {
		ip = strings.Trim(ip, " \n\t")
		if ip == "" {
			continue
		}
		m[ip] = struct{}{}
	}

	return m
}

func loadFieldsForDataStream(dataStreamRootPath string) ([]FieldDefinition, error) {
	fieldsDir := filepath.Join(dataStreamRootPath, "fields")
	files, err := filepath.Glob(filepath.Join(fieldsDir, "*.yml"))
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory with fields failed (path: %s)", fieldsDir)
	}

	var fields []FieldDefinition
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, errors.Wrap(err, "reading fields file failed")
		}

		var u []FieldDefinition
		err = yaml.Unmarshal(body, &u)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling field body failed")
		}
		fields = append(fields, u...)
	}
	return fields, nil
}

// ValidateDocumentBody validates the provided document body.
func (v *Validator) ValidateDocumentBody(body json.RawMessage) multierror.Error {
	var c common.MapStr
	err := json.Unmarshal(body, &c)
	if err != nil {
		var errs multierror.Error
		errs = append(errs, errors.Wrap(err, "unmarshalling document body failed"))
		return errs
	}

	errs := v.validateMapElement("", c)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ValidateDocumentMap validates the provided document as common.MapStr.
func (v *Validator) ValidateDocumentMap(body common.MapStr) multierror.Error {
	errs := v.validateMapElement("", body)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (v *Validator) validateMapElement(root string, elem common.MapStr) multierror.Error {
	var errs multierror.Error
	for name, val := range elem {
		key := strings.TrimLeft(root+"."+name, ".")

		switch val := val.(type) {
		case []map[string]interface{}:
			for _, m := range val {
				err := v.validateMapElement(key, m)
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
			err := v.validateMapElement(key, val)
			if err != nil {
				errs = append(errs, err...)
			}
		default:
			err := v.validateScalarElement(key, val)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (v *Validator) validateScalarElement(key string, val interface{}) error {
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

	if !v.disabledDependencyManagement && definition.External != "" {
		def, err := v.FieldDependencyManager.ImportField(definition.External, key)
		if err != nil {
			return errors.Wrapf(err, "can't import field (field: %s)", key)
		}
		definition = &def
	}

	// Convert numeric keyword fields to string for validation.
	_, found := v.numericKeywordFields[key]
	if (found || v.defaultNumericConversion) && isNumericKeyword(*definition, val) {
		val = fmt.Sprintf("%q", val)
	}

	err := v.parseElementValue(key, *definition, val)
	if err != nil {
		return errors.Wrap(err, "parsing field value failed")
	}
	return nil
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

		if len(def.Fields) == 0 {
			continue
		}

		fd := findElementDefinitionForRoot(key, searchedKey, def.Fields)
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

func compareKeys(key string, def FieldDefinition, searchedKey string) bool {
	k := strings.ReplaceAll(key, ".", "\\.")
	k = strings.ReplaceAll(k, "*", "[^.]+")

	// Workaround for potential geo_point, as "lon" and "lat" fields are not present in field definitions.
	// Unfortunately we have to assume that imported field could be a geo_point (nasty workaround).
	if def.Type == "geo_point" || def.External != "" {
		k += "(\\.lon|\\.lat|)"
	}

	k = fmt.Sprintf("^%s$", k)
	matched, err := regexp.MatchString(k, searchedKey)
	if err != nil {
		panic(errors.Wrapf(err, "regexp built using the given field/key (%s) is invalid", k))
	}
	return matched
}

func (v *Validator) parseElementValue(key string, definition FieldDefinition, val interface{}) error {
	val, ok := ensureSingleElementValue(val)
	if !ok {
		return nil // it's an array, but it's not possible to extract the single value.
	}

	var valid bool
	switch definition.Type {
	case "constant_keyword":
		var valStr string
		valStr, valid = val.(string)
		if !valid {
			break
		}

		if err := ensureConstantKeywordValueMatches(key, valStr, definition.Value); err != nil {
			return err
		}
		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}
	case "date", "keyword", "text":
		var valStr string
		valStr, valid = val.(string)
		if !valid {
			break
		}

		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}
	case "ip":
		var valStr string
		valStr, valid = val.(string)
		if !valid {
			break
		}

		if err := ensurePatternMatches(key, valStr, definition.Pattern); err != nil {
			return err
		}

		if v.enabledAllowedIPCheck && !v.isAllowedIPValue(valStr) {
			return fmt.Errorf("the IP %q is not one of the allowed test IPs (see: https://github.com/elastic/elastic-package/blob/main/internal/fields/_static/allowed_geo_ips.txt)", valStr)
		}
	case "float", "long", "double":
		_, valid = val.(float64)
	default:
		valid = true // all other types are considered valid not blocking validation
	}

	if !valid {
		return fmt.Errorf("field %q's Go type, %T, does not match the expected field type: %s (field value: %v)", key, val, definition.Type, val)
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
	if _, found := v.allowedIPs[s]; found {
		return true
	}

	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}

	if ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	return false
}

// ensureSingleElementValue extracts single entity from a potential array, which is a valid field representation
// in Elasticsearch. For type assertion we need a single value.
func ensureSingleElementValue(val interface{}) (interface{}, bool) {
	arr, isArray := val.([]interface{})
	if !isArray {
		return val, true
	}
	if len(arr) > 0 {
		return arr[0], true
	}
	return nil, false // false: empty array, can't deduce single value type
}

// ensurePatternMatches validates the document's field value matches the field
// definitions regular expression pattern.
func ensurePatternMatches(key, value, pattern string) error {
	if pattern == "" {
		return nil
	}
	valid, err := regexp.MatchString(pattern, value)
	if err != nil {
		return errors.Wrap(err, "invalid pattern")
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
