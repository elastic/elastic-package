// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

const terraformDeployerRun = `#!sh

set -euxo pipefail

cp -r /stage/*.tf /workspace

cleanup() {
  r=$?

  set -x
  terraform destroy -auto-approve

  exit $r
}
trap cleanup EXIT INT TERM

terraform init
terraform plan
terraform apply -auto-approve && touch /tmp/tf-applied

echo "Terraform definitions applied."

set +x
while true; do sleep 1; done # wait for ctrl-c
`
