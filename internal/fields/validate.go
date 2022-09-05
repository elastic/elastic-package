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
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
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
	allowedCIDRs          []*net.IPNet
}

// ValidatorOption represents an optional flag that can be passed to  CreateValidatorForDirectory.
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

// CreateValidatorForDirectory function creates a validator for the directory.
func CreateValidatorForDirectory(fieldsParentDir string, opts ...ValidatorOption) (v *Validator, err error) {
	v = new(Validator)
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	v.allowedCIDRs = initializeAllowedCIDRsList()

	fieldsDir := filepath.Join(fieldsParentDir, "fields")
	v.Schema, err = loadFieldsFromDir(fieldsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "can't load fields from directory (path: %s)", fieldsDir)
	}

	if v.disabledDependencyManagement {
		return v, nil
	}

	packageRoot, found, err := packages.FindPackageRoot()
	if err != nil {
		return nil, errors.Wrap(err, "can't find package root")
	}
	// As every command starts with approximating where is the package root, it isn't required to return an error in case the root is missing.
	// This is also useful for testing purposes, where we don't have a real package, but just "fields" directory. The package root is always absent.
	if !found {
		logger.Debug("Package root not found, dependency management will be disabled.")
		v.disabledDependencyManagement = true
		return v, nil
	}

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

func loadFieldsFromDir(fieldsDir string) ([]FieldDefinition, error) {
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
func (v *Validator) parseElementValue(key string, definition FieldDefinition, val interface{}) error {
	return forEachElementValue(key, definition, val, v.parseSingleElementValue)
}

func (v *Validator) parseSingleElementValue(key string, definition FieldDefinition, val interface{}) error {
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
func forEachElementValue(key string, definition FieldDefinition, val interface{}, fn func(string, FieldDefinition, interface{}) error) error {
	arr, isArray := val.([]interface{})
	if !isArray {
		return fn(key, definition, val)
	}
	for _, element := range arr {
		err := fn(key, definition, element)
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
