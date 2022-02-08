#!/usr/bin/env bash

set -euo pipefail

# Terraform code may rely on content from other files than .tf files (es json, zip, html, text), so we copy all the content over
# See more: https://github.com/elastic/elastic-package/pull/603
cp -r /stage/* /workspace

cleanup() {
  r=$?

  terraform destroy -auto-approve

  exit $r
}
trap cleanup EXIT INT TERM

gcp_auth() {
  if test -n "$(printenv "GOOGLE_CREDENTIALS")"; then
    # Save GCP credentials on disk and perform authentication
    # NOTE: this is required for bq (and maybe other gcloud related tools) to authenticate
    export "GOOGLE_APPLICATION_CREDENTIALS=/root/.config/gcloud/application_default_credentials.json"
    printenv "GOOGLE_CREDENTIALS" > "$GOOGLE_APPLICATION_CREDENTIALS"
    gcloud auth login --cred-file "$GOOGLE_APPLICATION_CREDENTIALS"
    # NOTE: Terraform support authentication through GOOGLE_CREDENTIALS and usual gcloud ADC but other
    # tools (like bq) don't support the first, so we always rely on gcloud ADC.
    unset "GOOGLE_CREDENTIALS"
  fi
}

terraform init
terraform plan
terraform apply -auto-approve && touch /tmp/tf-applied

echo "Terraform definitions applied."

while true; do sleep 1; done # wait for ctrl-c
