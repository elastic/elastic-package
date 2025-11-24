// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/multierror"
)

type results struct {
	Expected []json.RawMessage
}

type rootTestFinder struct {
	packageRoot    string
	repositoryRoot string
}

func (p rootTestFinder) FindPackageRoot() (string, error) {
	return p.packageRoot, nil
}

func (p rootTestFinder) FindRepositoryRoot() (*os.Root, error) {
	return os.OpenRoot(p.repositoryRoot)
}

func TestValidate_NoWildcardFields(t *testing.T) {
	finder := rootTestFinder{
		packageRoot:    "../../test/packages/parallel/aws/data_stream/elb_logs",
		repositoryRoot: "../../test/packages",
	}
	validator, err := createValidatorForDirectoryAndPackageRoot("../../test/packages/parallel/aws/data_stream/elb_logs",
		finder,
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	f := readTestResults(t, "../../test/packages/parallel/aws/data_stream/elb_logs/_dev/test/pipeline/test-alb.log-expected.json")
	for _, e := range f.Expected {
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	}
}

func TestValidate_WithWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/parallel/aws/data_stream/sns", WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/parallel/aws/data_stream/sns/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithFlattenedFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/flattened.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_ObjectTypeWithoutWildcard(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	t.Run("subobjects", func(t *testing.T) {
		e := readSampleEvent(t, "testdata/subobjects.json")
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	})

	t.Run("no-subobjects", func(t *testing.T) {
		e := readSampleEvent(t, "testdata/no-subobjects.json")
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	})
}

func TestValidate_DisabledParent(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	t.Run("disabled", func(t *testing.T) {
		e := readSampleEvent(t, "testdata/disabled.json")
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	})
}

func TestValidate_EnabledNotMappedError(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	t.Run("enabled", func(t *testing.T) {
		e := readSampleEvent(t, "testdata/enabled_not_mapped.json")
		errs := validator.ValidateDocumentBody(e)
		if assert.Len(t, errs, 2) {
			for i := 0; i < 2; i++ {
				assert.Contains(t, []string{`field "enabled.id" is undefined`, `field "enabled.status" is undefined`}, errs[i].Error())
			}
		}
	})
}

func TestValidate_WithNumericKeywordFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithNumericKeywordFields([]string{
			"foo.code", // Contains a number.
			"foo.pid",  // Contains an array of numbers.
			"foo.ppid", // Contains an empty array.
			"tags",     // Contains an empty array, and expects normalization as array.
		}),
		WithSpecVersion("2.3.0"), // Needed to validate normalization.
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/numeric.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithStringNumberFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithStringNumberFields([]string{
			"foo.count",  // Contains a number as string.
			"foo.metric", // Contains a floating number as string.
		}),
		WithSpecVersion("2.3.0"), // Needed to validate normalization.
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/stringnumbers.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithEnabledImportAllECSSchema(t *testing.T) {
	finder := rootTestFinder{
		packageRoot:    "../../test/packages/other/imported_mappings_tests",
		repositoryRoot: "../../test/packages",
	}
	validator, err := createValidatorForDirectoryAndPackageRoot("../../test/packages/other/imported_mappings_tests/data_stream/first",
		finder,
		WithSpecVersion("2.3.0"),
		WithEnabledImportAllECSSChema(true))
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/imported_mappings_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithDisabledImportAllECSSchema(t *testing.T) {
	finder := rootTestFinder{
		packageRoot:    "../../test/packages/other/imported_mappings_tests",
		repositoryRoot: "../../test/packages",
	}
	validator, err := createValidatorForDirectoryAndPackageRoot("../../test/packages/other/imported_mappings_tests/data_stream/first",
		finder,
		WithSpecVersion("2.3.0"),
		WithEnabledImportAllECSSChema(false))
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/imported_mappings_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 4)

	errorMessages := []string{}
	for _, err := range errs {
		errorMessages = append(errorMessages, err.Error())
	}
	sort.Strings(errorMessages)
	require.Contains(t, errorMessages[0], `field "destination.geo.location.lat" is undefined`)
}

func TestValidate_constantKeyword(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/constant-keyword-invalid.json")
	errs := validator.ValidateDocumentBody(e)
	require.NotEmpty(t, errs)

	e = readSampleEvent(t, "testdata/constant-keyword-valid.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_ipAddress(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithEnabledAllowedIPCheck(), WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/ip-address-forbidden.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), `the IP "98.76.54.32" is not one of the allowed test IPs`)

	e = readSampleEvent(t, "testdata/ip-address-allowed.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_undefinedArrayOfObjects(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithSpecVersion("2.0.0"), WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/undefined-array-of-objects.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), `field "user.group" is used as array of objects, expected explicit definition with type group or nested`)
}

func TestValidate_WithSpecVersion(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithSpecVersion("2.0.0"), WithDisabledDependencyManagement())
	require.NoError(t, err)

	e := readSampleEvent(t, "testdata/invalid-array-normalization.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), `field "container.image.tag" is not normalized as expected`)

	e = readSampleEvent(t, "testdata/valid-array-normalization.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)

	// Check now that this validation was only enabled for 2.0.0.
	validator, err = CreateValidatorForDirectory("testdata", WithSpecVersion("1.99.99"), WithDisabledDependencyManagement())
	require.NoError(t, err)

	e = readSampleEvent(t, "testdata/invalid-array-normalization.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_ExpectedEventType(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithSpecVersion("2.0.0"), WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	cases := []struct {
		title string
		doc   common.MapStr
		valid bool
	}{
		{
			title: "valid event type",
			doc: common.MapStr{
				"event.category": "authentication",
				"event.type":     []any{"info"},
			},
			valid: true,
		},
		{
			title: "no event type",
			doc: common.MapStr{
				"event.category": "authentication",
			},
			valid: true,
		},
		{
			title: "multiple valid event type",
			doc: common.MapStr{
				"event.category": "network",
				"event.type":     []any{"protocol", "connection", "end"},
			},
			valid: true,
		},
		{
			title: "multiple categories",
			doc: common.MapStr{
				"event.category": []any{"iam", "configuration"},
				"event.type":     []any{"group", "change"},
			},
			valid: true,
		},
		{
			title: "unexpected event type",
			doc: common.MapStr{
				"event.category": "authentication",
				"event.type":     []any{"access"},
			},
			valid: false,
		},
		{
			title: "multiple categories, no match",
			doc: common.MapStr{
				"event.category": []any{"iam", "configuration"},
				"event.type":     []any{"denied", "end"},
			},
			valid: false,
		},
		{
			title: "multiple categories, some types don't match",
			doc: common.MapStr{
				"event.category": []any{"iam", "configuration"},
				"event.type":     []any{"denied", "end", "group", "change"},
			},
			valid: false,
		},
		{
			title: "multi-field",
			doc: common.MapStr{
				"process.name":      "elastic-package",
				"process.name.text": "elastic-package",
			},
			valid: true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			errs := validator.ValidateDocumentMap(c.doc)
			if c.valid {
				assert.Empty(t, errs, "should not have errors")
			} else {
				if assert.Len(t, errs, 1, "should have one error") {
					assert.Contains(t, errs[0].Error(), "is not one of the expected values")
				}
			}
		})
	}
}

func TestValidate_ExpectedDatasets(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithSpecVersion("2.0.0"),
		WithExpectedDatasets([]string{"apache.status"}),
		WithDisabledDependencyManagement(),
	)
	require.NoError(t, err)
	require.NotNil(t, validator)

	cases := []struct {
		title string
		doc   common.MapStr
		valid bool
	}{
		{
			title: "valid dataset",
			doc: common.MapStr{
				"event.dataset": "apache.status",
			},
			valid: true,
		},
		{
			title: "empty dataset",
			doc: common.MapStr{
				"event.dataset": "",
			},
			valid: false,
		},
		{
			title: "absent dataset",
			doc:   common.MapStr{},
			valid: true,
		},
		{
			title: "wrong dataset",
			doc: common.MapStr{
				"event.dataset": "httpd.status",
			},
			valid: false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			errs := validator.ValidateDocumentMap(c.doc)
			if c.valid {
				assert.Empty(t, errs)
			} else {
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `field "event.dataset" should have value`)
				}
			}
		})
	}
}

func Test_parseElementValue(t *testing.T) {
	for _, test := range []struct {
		key         string
		value       any
		definition  FieldDefinition
		fail        bool
		assertError func(t *testing.T, err error)
		specVersion semver.Version
	}{
		// Arrays
		{
			key:   "string array to keyword",
			value: []any{"hello", "world"},
			definition: FieldDefinition{
				Type: "keyword",
			},
		},
		{
			key:   "mixed numbers and strings in number array",
			value: []any{123, "hi"},
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
		},

		// keyword and constant_keyword (string)
		{
			key:   "constant_keyword with pattern",
			value: "some value",
			definition: FieldDefinition{
				Type:    "constant_keyword",
				Pattern: `^[a-z]+\s[a-z]+$`,
			},
		},
		{
			key:   "constant_keyword fails pattern",
			value: "some value",
			definition: FieldDefinition{
				Type:    "constant_keyword",
				Pattern: `[0-9]`,
			},
			fail: true,
		},
		// keyword and constant_keyword (other)
		{
			key:   "bad type for keyword",
			value: map[string]any{},
			definition: FieldDefinition{
				Type: "keyword",
			},
			fail: true,
		},
		// date
		{
			key:   "date",
			value: "2020-11-02T18:01:03Z",
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
		},
		{
			key:   "date as milliseconds",
			value: float64(1420070400001),
			definition: FieldDefinition{
				Type: "date",
			},
		},
		{
			key:   "date as milisecond with pattern",
			value: float64(1420070400001),
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
			fail: true,
		},
		{
			key:   "bad date",
			value: "10 Oct 2020 3:42PM",
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
			fail: true,
		},
		// ip
		{
			key:   "ip",
			value: "127.0.0.1",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "^[0-9.]+$",
			},
		},
		{
			key:   "bad ip",
			value: "localhost",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "^[0-9.]+$",
			},
			fail: true,
		},
		{
			key:   "ip in allowed list",
			value: "1.128.3.4",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 in allowed list",
			value: "2a02:cf40:add:4002:91f2:a9b2:e09a:6fc6",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "unspecified ipv6",
			value: "::",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "unspecified ipv4",
			value: "0.0.0.0",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv4 broadcast address",
			value: "255.255.255.255",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 min multicast",
			value: "ff00::",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 max multicast",
			value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "abbreviated ipv6 in allowed list with leading 0",
			value: "2a02:cf40:0add:0::1",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ip not in geoip database",
			value: "8.8.8.8",
			definition: FieldDefinition{
				Type: "ip",
			},
			fail: true,
		},
		// text
		{
			key:   "text",
			value: "some text",
			definition: FieldDefinition{
				Type: "text",
			},
		},
		{
			key:   "text with pattern",
			value: "more text",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "[A-Z]",
			},
			fail: true,
		},
		// float
		{
			key:   "float",
			value: 3.1416,
			definition: FieldDefinition{
				Type: "float",
			},
		},
		// allowed values
		{
			key:   "allowed values",
			value: "configuration",
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
		},
		{
			key:   "not allowed value",
			value: "display",
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
			fail: true,
		},
		{
			key:   "not allowed value in array",
			value: []string{"configuration", "display"},
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
			fail: true,
		},
		// expected values
		{
			key:   "expected values",
			value: "linux",
			definition: FieldDefinition{
				Type:           "keyword",
				ExpectedValues: []string{"linux", "windows"},
			},
		},
		{
			key:   "not expected values",
			value: "bsd",
			definition: FieldDefinition{
				Type:           "keyword",
				ExpectedValues: []string{"linux", "windows"},
			},
			fail: true,
		},
		// fields shouldn't be stored in groups
		{
			key:   "host",
			value: "42",
			definition: FieldDefinition{
				Type: "group",
			},
			fail: true,
		},
		// arrays of objects can be stored in groups, even if not recommended
		{
			key: "host",
			value: []any{
				map[string]any{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
				map[string]any{
					"id":       "otherhost-id",
					"hostname": "otherhost",
				},
			},
			definition: FieldDefinition{
				Name: "host",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
					{
						Name: "hostname",
						Type: "keyword",
					},
				},
			},
		},
		// elements in arrays of objects should be validated
		{
			key: "details",
			value: []any{
				map[string]any{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
			},
			definition: FieldDefinition{
				Name: "details",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_1,
			fail:        true,
			assertError: func(t *testing.T, err error) {
				errs := err.(multierror.Error)
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `"details.hostname" is undefined`)
				}
			},
		},
		// elements in nested objects
		{
			key: "nested",
			value: []any{
				map[string]any{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
			},
			definition: FieldDefinition{
				Name: "nested",
				Type: "nested",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_1,
			fail:        true,
			assertError: func(t *testing.T, err error) {
				errs := err.(multierror.Error)
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `"nested.hostname" is undefined`)
				}
			},
		},
		// arrays of elements in nested objects
		{
			key: "good_array_of_nested",
			value: []any{
				[]any{
					map[string]any{
						"id":       "somehost-id",
						"hostname": "somehost",
					},
				},
			},
			definition: FieldDefinition{
				Name: "good_array_of_nested",
				Type: "nested",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
					{
						Name: "hostname",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_1,
		},
		{
			key: "array_of_nested",
			value: []any{
				[]any{
					map[string]any{
						"id":       "somehost-id",
						"hostname": "somehost",
					},
				},
			},
			definition: FieldDefinition{
				Name: "array_of_nested",
				Type: "nested",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_1,
			fail:        true,
			assertError: func(t *testing.T, err error) {
				var errs multierror.Error
				require.ErrorAs(t, err, &errs)
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `"array_of_nested.hostname" is undefined`)
				}
			},
		},
		{
			key:   "null_array",
			value: nil,
			definition: FieldDefinition{
				Name: "null_array",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_1,
		},
	} {

		t.Run(test.key, func(t *testing.T) {
			v := Validator{
				Schema:                       []FieldDefinition{test.definition},
				disabledDependencyManagement: true,
				enabledAllowedIPCheck:        true,
				allowedCIDRs:                 initializeAllowedCIDRsList(),
				specVersion:                  test.specVersion,
			}

			errs := v.parseElementValue(test.key, test.definition, test.value, common.MapStr{})
			if test.fail {
				assert.Greater(t, len(errs), 0)
				if test.assertError != nil {
					test.assertError(t, errs)
				}
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestCompareKeys(t *testing.T) {
	cases := []struct {
		key         string
		def         FieldDefinition
		searchedKey string
		expected    bool
	}{
		{
			key:         "example.foo",
			searchedKey: "example.foo",
			expected:    true,
		},
		{
			key:         "example.bar",
			searchedKey: "example.foo",
			expected:    false,
		},
		{
			key:         "example.foo",
			searchedKey: "example.foos",
			expected:    false,
		},
		{
			key:         "example.foo",
			searchedKey: "example.fo",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.foo",
			expected:    true,
		},
		{
			key:         "example.foo",
			searchedKey: "example.*",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.",
			expected:    false,
		},
		{
			key:         "example.*.foo",
			searchedKey: "example.group.foo",
			expected:    true,
		},
		{
			key:         "example.*.*",
			searchedKey: "example.group.foo",
			expected:    true,
		},
		{
			key:         "example.*.*",
			searchedKey: "example..foo",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.group.foo",
			expected:    false,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lat",
			expected:    true,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.foo",
			expected:    false,
		},
		{
			key:         "example.ecs.geo",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.ecs.geo.lat",
			expected:    true,
		},
		{
			key:         "example.ecs.geo",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.ecs.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.geo.lat",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "object", ObjectType: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.foo",
			expected:    false,
		},
		{
			key:         "example.histogram",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.counts",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.counts",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.values",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.foo",
			expected:    false,
		},
	}

	for _, c := range cases {
		t.Run(c.key+" matches "+c.searchedKey, func(t *testing.T) {
			found := compareKeys(c.key, c.def, c.searchedKey)
			assert.Equal(t, c.expected, found)
		})
	}
}

func TestValidateGeoPoint(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/other/fields_tests/data_stream/first", WithDisabledDependencyManagement())

	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/fields_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidateExternalMultiField(t *testing.T) {
	packageRoot := "../../test/packages/parallel/mongodb"
	dataStreamRoot := filepath.Join(packageRoot, "data_stream", "status")

	validator, err := createValidatorForDirectoryAndPackageRoot(dataStreamRoot,
		rootTestFinder{packageRoot, "../../test/packages"})
	require.NoError(t, err)
	require.NotNil(t, validator)

	def := FindElementDefinition("process.name", validator.Schema)
	require.NotEmpty(t, def.MultiFields, "expected to test with a data stream with a field with multifields")

	e := readSampleEvent(t, "testdata/mongodb-multi-fields.json")
	var event common.MapStr
	err = json.Unmarshal(e, &event)
	require.NoError(t, err)

	v, err := event.GetValue("process.name.text")
	require.NotNil(t, v, "expected document with multi-field")
	require.NoError(t, err, "expected document with multi-field")

	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidateStackVersionsWithEcsMappings(t *testing.T) {
	// List of unique stack constraints extracted from the
	// package manifest files in the elastic/integrations
	// repository.
	constraints := []struct {
		Constraints string
		SupportEcs  bool
	}{
		{"^7.17.0", false},
		{"7.17.19 || > 8.13", false},
		{"^7.14.0 || ^8.0.0", false},
		{"^7.14.1 || ^8.0.0", false},
		{"^7.14.1 || ^8.8.0", false},
		{"^7.16.0 || ^8.0.0", false},
		{"^7.17.0 || ^8.0.0", false},
		{"^8.0.0", false},
		{"^8.10.1", false},
		{"^8.10.2", false},
		{"^8.11.0", false},
		{"^8.11.2", false},
		{"^8.12.0", false},
		{"^8.12.1", false},
		{"^8.12.2", false},
		{"^8.13.0", true},
		{"^8.14.0", true},
		{"^8.2.0", false},
		{"^8.2.1", false},
		{"^8.3.0", false},
		{"^8.4.0", false},
		{"^8.5.0", false},
		{"^8.5.1", false},
		{"^8.6.0", false},
		{"^8.6.1", false},
		{"^8.7.0", false},
		{"^8.7.1", false},
		{"^8.8.0", false},
		{"^8.8.1", false},
		{"^8.8.2", false},
		{"^8.9.0", false},
		{">= 8.0.0, < 8.10.0", false},
		{">= 8.0.0, < 8.0.1", false},
	}

	for _, c := range constraints {
		constraint, err := semver.NewConstraint(c.Constraints)
		if err != nil {
			require.NoError(t, err)
		}
		assert.Equal(t, c.SupportEcs, allVersionsIncludeECS(constraint), "constraint: %s", c.Constraints)
	}
}

func TestSkipLeafOfObject(t *testing.T) {
	schema := []FieldDefinition{
		{
			Name: "foo",
			Type: "keyword",
		},
		{
			Name: "flattened",
			Type: "flattened",
		},
		{
			Name: "object",
			Type: "object",
		},
		{
			Name: "nested",
			Type: "nested",
		},
		{
			Name: "group",
			Type: "group",
			Fields: []FieldDefinition{
				{
					Name: "subgroup",
					Type: "object",
				},
			},
		},
	}

	cases := []struct {
		name     string
		version  *semver.Version
		expected bool
	}{
		{
			name:     "foo.bar",
			version:  semver.MustParse("3.0.0"),
			expected: true,
		},
		{
			name:     "subgroup.bar",
			version:  semver.MustParse("3.0.0"),
			expected: true,
		},
		{
			name:     "foo.bar",
			version:  semver.MustParse("3.0.1"),
			expected: false,
		},
		{
			name:     "subgroup.bar",
			version:  semver.MustParse("3.0.1"),
			expected: false,
		},
	}

	// Cases we expect to skip depending on the version.
	okRoots := []string{"flattened", "object", "group", "nested"}
	for _, root := range okRoots {
		t.Run("empty root with prefix "+root, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.name+"_"+c.version.String(), func(t *testing.T) {
					found := skipLeafOfObject("", root+"."+c.name, *c.version, schema)
					assert.Equal(t, c.expected, found)
				})
			}
		})
		t.Run(root, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.name+"_"+c.version.String(), func(t *testing.T) {
					found := skipLeafOfObject(root, c.name, *c.version, schema)
					assert.Equal(t, c.expected, found)
				})
			}
		})
	}

	// Cases we never expect to skip.
	notOkRoots := []string{"foo", "notexists", "group.subgroup.other"}
	for _, root := range notOkRoots {
		t.Run("not ok "+root, func(t *testing.T) {
			for _, c := range cases {
				t.Run(c.name+"_"+c.version.String(), func(t *testing.T) {
					found := skipLeafOfObject(root, c.name, *c.version, schema)
					assert.Equal(t, false, found)
				})
			}
		})
	}
}

func readTestResults(t *testing.T, path string) (f results) {
	c, err := os.ReadFile(path)
	require.NoError(t, err)

	err = json.Unmarshal(c, &f)
	require.NoError(t, err)
	return
}

func readSampleEvent(t *testing.T, path string) json.RawMessage {
	c, err := os.ReadFile(path)
	require.NoError(t, err)
	return c
}

func Test_IsAllowedIPValue(t *testing.T) {
	cases := []struct {
		title      string
		ip         string
		allowedIps []string
		expected   bool
	}{
		{
			title:    "private ipv4",
			ip:       "192.168.1.2",
			expected: true,
		},
		{
			title:    "private ipv4 other range",
			ip:       "10.2.2.2",
			expected: true,
		},
		{
			title:    "documentation IPv4",
			ip:       "192.0.2.10",
			expected: true,
		},
		{
			title:    "documentation IPv6",
			ip:       "2001:0DB8:1000:1000:1000:1000:1000:1000",
			expected: true,
		},
		{
			title:    "unspecified ipv4",
			ip:       "0.0.0.0",
			expected: true,
		},
		{
			title:    "unspecified ipv6",
			ip:       "0:0:0:0:0:0:0:0",
			expected: true,
		},
		{
			title:    "ip allowed CIDR",
			ip:       "89.160.20.115",
			expected: true,
			allowedIps: []string{
				"89.160.20.112/28",
			},
		},
		{
			title:    "not valid ipv4",
			ip:       "216.160.83.57",
			expected: false,
			allowedIps: []string{
				"89.160.20.112/28",
			},
		},
		{
			title:    "not valid ipv6",
			ip:       "2002:2002:1000:1000:1000:1000:1000:1000",
			expected: false,
			allowedIps: []string{
				"89.160.20.112/28",
			},
		},
		{
			title:      "valid ipv4 multicast address",
			ip:         "233.252.0.57",
			expected:   true,
			allowedIps: []string{},
		},
		{
			title:      "second range documentation ipv6",
			ip:         "3fff:0000:0000:0000:0000:1000:1000:1000",
			expected:   true,
			allowedIps: []string{},
		},
		{
			title:      "other invalid ipv6",
			ip:         "3fff:1fff:ffff:ffff:ffff:ffff:ffff:ffff",
			expected:   false,
			allowedIps: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			allowedCIDRs := []*net.IPNet{}
			for _, cidr := range c.allowedIps {
				_, cidr, err := net.ParseCIDR(cidr)
				require.NoError(t, err)
				allowedCIDRs = append(allowedCIDRs, cidr)
			}
			v := Validator{
				disabledDependencyManagement: true,
				enabledAllowedIPCheck:        true,
				allowedCIDRs:                 allowedCIDRs,
			}

			allowed := v.isAllowedIPValue(c.ip)
			assert.Equal(t, c.expected, allowed)
		})
	}

}
