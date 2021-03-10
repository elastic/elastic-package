// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const terraformDeployerYml = `version: '2.3'
services:
  terraform:
    build: .
    tty: true
    environment:
      - TF_VAR_TEST_RUN_ID=${TF_VAR_TEST_RUN_ID:-detached}
    volumes:
      - ${TF_DIR}:/stage
`
