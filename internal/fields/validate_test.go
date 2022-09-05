// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type results struct {
	Expected []json.RawMessage
}

func TestValidate_NoWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/parallel/aws/data_stream/elb_logs")
	require.NoError(t, err)
	require.NotNil(t, validator)

	f := readTestResults(t, "../../test/packages/parallel/aws/data_stream/elb_logs/_dev/test/pipeline/test-alb.log-expected.json")
	for _, e := range f.Expected {
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	}
}

func TestValidate_WithWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/parallel/aws/data_stream/sns")
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

func TestValidate_WithNumericKeywordFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithNumericKeywordFields([]string{"foo.code"}),
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/numeric.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_constantKeyword(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata")
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
	validator, err := CreateValidatorForDirectory("testdata", WithEnabledAllowedIPCheck())
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

func Test_parseElementValue(t *testing.T) {
	for _, test := range []struct {
		key        string
		value      interface{}
		definition FieldDefinition
		fail       bool
	}{
		// Arrays
		{
			key:   "string array to keyword",
			value: []interface{}{"hello", "world"},
			definition: FieldDefinition{
				Type: "keyword",
			},
		},
		{
			key:   "numeric string array to long",
			value: []interface{}{"123", "42"},
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
		},
		{
			key:   "mixed numbers and strings in number array",
			value: []interface{}{123, "hi"},
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
			value: map[string]interface{}{},
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
		// long
		{
			key:   "bad long",
			value: "65537",
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
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
			value: []interface{}{
				map[string]interface{}{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
				map[string]interface{}{
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
	} {
		v := Validator{
			disabledDependencyManagement: true,
			enabledAllowedIPCheck:        true,
			allowedCIDRs:                 initializeAllowedCIDRsList(),
		}

		t.Run(test.key, func(t *testing.T) {
			err := v.parseElementValue(test.key, test.definition, test.value)
			if test.fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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

func TestValidate_geo_point(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/other/fields_tests/data_stream/first")

	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/fields_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}
