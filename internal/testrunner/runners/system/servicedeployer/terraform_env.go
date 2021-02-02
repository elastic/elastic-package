// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
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

func (tsd TerraformServiceDeployer) buildTerraformAliases() map[string]interface{} {
	return map[string]interface{}{
		//TODO awsAccessKeyID:     os.Getenv(awsAccessKeyID),
	}
}
