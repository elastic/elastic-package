// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"fmt"

	"github.com/elastic/elastic-package/internal/compose"
)

const (
	tfDir       = "TF_DIR"
	tfTestRunID = "TF_VAR_TEST_RUN_ID"

	envYmlFile = "env.yml"
)

func (tsd TerraformServiceDeployer) buildTerraformExecutorEnvironment(ctxt ServiceContext) []string {
	vars := map[string]string{}
	vars[serviceLogsDirEnv] = ctxt.Logs.Folder.Local
	vars[tfTestRunID] = ctxt.Test.RunID
	vars[tfDir] = tsd.definitionsDir

	var pairs []string
	for k, v := range vars {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return pairs
}

func buildTerraformAliases(serviceComposeConfig *compose.Config) (map[string]interface{}, error) {
	terraformService, found := serviceComposeConfig.Services["terraform"]
	if !found {
		return nil, errors.New("missing config section for terraform service")
	}

	m := map[string]interface{}{}
	for k, v := range terraformService.Environment {
		// skip empty variables and the internal alias for test run
		if v != "" && v != tfTestRunID {
			m[k] = v
		}
	}
	return m, nil
}
