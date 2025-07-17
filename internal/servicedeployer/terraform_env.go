// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/compose"
)

const (
	tfDir       = "TF_DIR"
	tfOutputDir = "TF_OUTPUT_DIR"
	tfTestRunID = "TF_VAR_TEST_RUN_ID"

	envYmlFile = "env.yml"
)

func (tsd TerraformServiceDeployer) buildTerraformExecutorEnvironment(info ServiceInfo) []string {
	vars := map[string]string{}
	vars[serviceLogsDirEnv] = info.Logs.Folder.Local
	vars[tfTestRunID] = info.Test.RunID
	vars[tfDir] = tsd.definitionsDir
	vars[tfOutputDir] = info.OutputDir

	// if v, found := os.LookupEnv("ELASTIC_PACKAGE_SET_TERRAFORM_RUN_ID"); found && v != "" {
	// 	vars[tfTestRunID] = v
	// }

	// if v, found := os.LookupEnv("ELASTIC_PACKAGE_PREFIX_TERRAFORM_RUN_ID"); found && v != "" {
	// 	vars[tfTestRunID] = fmt.Sprintf("%s%s", v, info.Test.RunID)
	// }

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
	for name, value := range terraformService.Environment {
		// skip empty values and internal Terraform variables
		if value != "" && !strings.HasPrefix(name, "TF_VAR_") {
			m[name] = value
		}
	}
	return m, nil
}
