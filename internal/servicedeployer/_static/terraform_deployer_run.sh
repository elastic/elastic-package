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

  if [[ "${running_on_aws}" == 1 ]]; then
    echo "After Terraform destroy command"
    aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'
  fi

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

running_on_aws=0
if [[ "${AWS_SECRET_ACCESS_KEY:-""}" != "" ]]; then
  running_on_aws=1
  echo "Before Terraform Apply command"
  aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'

  buckets=(
      "elastic-package-canva-bucket-64363"
      "elastic-package-canva-bucket-51662"
      "elastic-package-sublime-security-bucket-35776"
      "elastic-package-symantec-endpoint-security-bucket-65009"
      "elastic-package-symantec-endpoint-security-bucket-78346"
  )
  for b in "${buckets[@]}"; do
      echo "Check buckets: ${b}"
      aws s3api head-bucket --bucket "${b}"
      echo ""
  done
fi


retry 2 terraform apply -auto-approve

if [[ "${running_on_aws}" == 1 ]]; then
  echo "After Terraform Apply command"
  aws s3api list-buckets --query "Buckets[].Name" --output text | tr '\t' '\n'
fi

terraform output -json > /output/tfOutputValues.json

touch /tmp/tf-applied # This file is used as indicator (healthcheck) that the service is UP, and so it must be placed as the last statement in the script

echo "Terraform definitions applied."

set +x
while true; do sleep 1; done # wait for ctrl-c
