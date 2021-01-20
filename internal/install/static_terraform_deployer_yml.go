// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const terraformDeployerYml = `version: '2.3'
services:
  aws_sns:
    build: .
    tty: true
    environment:
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - AWS_PROFILE=${AWS_PROFILE}
      - AWS_REGION=${AWS_REGION:-us-east-1}
    volumes:
      - ${TF_DIR}:/stage
`
