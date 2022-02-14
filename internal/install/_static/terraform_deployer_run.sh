#!/usr/bin/env bash

set -euxo pipefail

# Terraform code may rely on content from other files than .tf files (es json, zip, html, text), so we copy all the content over
# See more: https://github.com/elastic/elastic-package/pull/603
# NOTE: must copy hidden files too (supported by "/.")
# See more: https://github.com/elastic/package-spec/issues/269
cp -r /stage/. /workspace

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
