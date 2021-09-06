// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import _ "embed"

//go:embed _static/kibana_healthcheck.sh
var kibanaHealthcheckSh string

//go:embed _static/Dockerfile.terraform_deployer
var terraformDeployerDockerfile string

//go:embed _static/terraform_deployer.yml
var terraformDeployerYml string

//go:embed _static/terraform_deployer_run.sh
var terraformDeployerRun string
