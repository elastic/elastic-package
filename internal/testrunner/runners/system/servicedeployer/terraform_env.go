// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"os"
)

const (
	awsAccessKeyID     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	awsProfile         = "AWS_PROFILE"
	awsRegion          = "AWS_REGION"

	tfDir       = "TF_DIR"
	tfTestRunID = "TF_VAR_TEST_RUN_ID"
)

func (tsd TerraformServiceDeployer) buildTerraformExecutorEnvironment(ctxt ServiceContext) []string {
	vars := map[string]string{}
	vars[serviceLogsDirEnv] = ctxt.Logs.Folder.Local
	vars[tfTestRunID] = ctxt.Test.RunID
	vars[tfDir] = tsd.definitionsDir

	if os.Getenv(awsAccessKeyID) != "" && os.Getenv(awsSecretAccessKey) != "" {
		vars[awsAccessKeyID] = os.Getenv(awsAccessKeyID)
		vars[awsSecretAccessKey] = os.Getenv(awsSecretAccessKey)
	} else if os.Getenv(awsProfile) != "" {
		vars[awsProfile] = os.Getenv(awsProfile)
	}

	if os.Getenv(awsRegion) != "" {
		vars[awsRegion] = os.Getenv(awsRegion)
	}

	var pairs []string
	for k, v := range vars {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return pairs
}

func buildTerraformAliases() map[string]interface{} {
	return map[string]interface{}{
		awsAccessKeyID:     os.Getenv(awsAccessKeyID),
		awsSecretAccessKey: os.Getenv(awsSecretAccessKey),
		awsProfile:         os.Getenv(awsProfile),
		awsRegion:          os.Getenv(awsRegion),
	}
}
