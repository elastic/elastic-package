// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type results struct {
	Expected []json.RawMessage
}

func TestValidate_NoWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDataStream("../../test/packages/parallel/aws/data_stream/elb_logs")
	require.NoError(t, err)
	require.NotNil(t, validator)

	f := readTestResults(t, "../../test/packages/parallel/aws/data_stream/elb_logs/_dev/test/pipeline/test-alb.log-expected.json")
	for _, e := range f.Expected {
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	}
}

func TestValidate_WithWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDataStream("../../test/packages/parallel/aws/data_stream/sns")
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/parallel/aws/data_stream/sns/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithFlattenedFields(t *testing.T) {
	validator, err := CreateValidatorForDataStream("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/flattened.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithNumericKeywordFields(t *testing.T) {
	validator, err := CreateValidatorForDataStream("testdata",
		WithNumericKeywordFields([]string{"foo.code"}),
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/numeric.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_constantKeyword(t *testing.T) {
	validator, err := CreateValidatorForDataStream("testdata")
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
	validator, err := CreateValidatorForDataStream("testdata", WithEnabledAllowedIPCheck())
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
		// Arrays (only first value checked)
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
	} {
		v := Validator{disabledDependencyManagement: true}
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
	validator, err := CreateValidatorForDataStream("../../test/packages/other/fields_tests/data_stream/first")

	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/fields_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}
