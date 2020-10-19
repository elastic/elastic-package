// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields_test

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/fields"
)

type results struct {
	Expected []json.RawMessage
}

func TestValidate_NoWildcardFields(t *testing.T) {
	validator, err := fields.CreateValidatorForDataStream("../../test/packages/aws/data_stream/elb_logs")
	require.NoError(t, err)
	require.NotNil(t, validator)

	f := readTestResults(t, "../../test/packages/aws/data_stream/elb_logs/_dev/test/pipeline/test-alb.log-expected.json")
	for _, e := range f.Expected {
		err = validator.ValidateDocumentBody(e)
		require.NoError(t, err)
	}
}

func TestValidate_WithWildcardFields(t *testing.T) {
	validator, err := fields.CreateValidatorForDataStream("../../test/packages/aws/data_stream/sns")
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/aws/data_stream/sns/sample_event.json")
	err = validator.ValidateDocumentBody(e)
	require.NoError(t, err)
}

func readTestResults(t *testing.T, path string) (f results) {
	c, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	err = json.Unmarshal(c, &f)
	require.NoError(t, err)
	return
}

func readSampleEvent(t *testing.T, path string) json.RawMessage {
	c, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	return c
}
