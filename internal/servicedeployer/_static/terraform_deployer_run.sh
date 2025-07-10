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

  echo "After Terraform destroy command"
  aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'

  exit $r
}
trap cleanup EXIT INT TERM

terraform init
terraform plan

export AWS_DEFAULT_REGION="${AWS_REGION}"
echo "Before Terraform Apply command"
aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'

export TF_LOG="DEBUG"
terraform apply -auto-approve

echo "After Terraform Apply command"
aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'

terraform output -json > /output/tfOutputValues.json

touch /tmp/tf-applied # This file is used as indicator (healthcheck) that the service is UP, and so it must be placed as the last statement in the script

echo "Terraform definitions applied."

set +x
while true; do sleep 1; done # wait for ctrl-c
