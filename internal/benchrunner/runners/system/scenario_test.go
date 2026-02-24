// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig_DeployerValidation(t *testing.T) {
	var testCases = []struct {
		testName     string
		scenarioName string
		deployer     string
		errContains  string
	}{
		{
			testName:     "valid deployer docker",
			scenarioName: "valid_deployer_docker",
			deployer:     "docker",
		},
		{
			testName:     "valid deployer k8s",
			scenarioName: "valid_deployer_k8s",
			deployer:     "k8s",
		},
		{
			testName:     "valid deployer tf",
			scenarioName: "valid_deployer_tf",
			deployer:     "tf",
		},
		{
			testName:     "invalid deployer",
			scenarioName: "invalid_deployer",
			errContains:  "invalid deployer name",
		},
		{
			testName:     "empty deployer",
			scenarioName: "empty_deployer",
			deployer:     "",
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			scenario, err := readRawConfig("testdata", tc.scenarioName)

			if tc.errContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.deployer, scenario.Deployer)
		})
	}
}
