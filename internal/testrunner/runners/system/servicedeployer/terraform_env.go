// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import "fmt"

const (
	awsAccessKeyID     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	awsProfile         = "AWS_PROFILE"
	awsRegion          = "AWS_REGION"
)

func buildTerraformEnvironmentVars(ctxt ServiceContext) ([]string, error) {
	var vars []string
	vars = append(vars, fmt.Sprintf("%s=%s", serviceLogsDirEnv, ctxt.Logs.Folder.Local))

	// TODO load vars
	// TODO safe cmd prints

	return vars, nil
}
