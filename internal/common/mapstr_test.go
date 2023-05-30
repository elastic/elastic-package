// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapStrGetValue(t *testing.T) {

	cases := []struct {
		title         string
		testFile      string
		fieldKey      string
		expectedValue interface{}
	}{
		{
			title:         "string value",
			testFile:      "./testdata/source.json",
			fieldKey:      "host.architecture",
			expectedValue: "x86_64",
		},
		{
			title:         "float64 value",
			testFile:      "./testdata/source.json",
			fieldKey:      "metricset.period",
			expectedValue: float64(10000),
		},
		{
			title:         "slice value",
			testFile:      "./testdata/source.json",
			fieldKey:      "tags",
			expectedValue: []interface{}{"apache_tomcat-cache", "forwarded"},
		},
		{
			title:         "map value",
			testFile:      "./testdata/source.json",
			fieldKey:      "data_stream",
			expectedValue: map[string]interface{}{"dataset": "apache_tomcat.cache", "namespace": "ep", "type": "metrics"},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			b, err := os.ReadFile("./testdata/source.json")
			require.NoError(t, err)

			var given MapStr
			err = json.Unmarshal(b, &given)
			require.NoError(t, err)

			val, err := given.GetValue(c.fieldKey)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedValue, val)
		})
	}
}
