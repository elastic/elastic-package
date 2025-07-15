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

retry() {
  local retries=$1
  shift
  local count=0
  until "$@"; do
    exit=$?
    wait=$((2 ** count))
    count=$((count + 1))
    if [ $count -lt "$retries" ]; then
      >&2 echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
      sleep $wait
    else
      >&2 echo "Retry $count/$retries exited $exit, no more retries left."
      return $exit
    fi
  done
  return 0
}

terraform init
terraform plan

retry 2 terraform apply -auto-approve

terraform output -json > /output/tfOutputValues.json

touch /tmp/tf-applied # This file is used as indicator (healthcheck) that the service is UP, and so it must be placed as the last statement in the script

echo "Terraform definitions applied."

set +x
while true; do sleep 1; done # wait for ctrl-c
