#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

set -euo pipefail

AWS_RESOURCES_FILE="aws.resources.txt"
GCP_RESOURCES_FILE="gcp.resources.txt"

# TODO: change to 24 hours ago
export CREATION_DATE=$(date -Is -d "1 minute ago")

resource_to_delete=0

any_resources_to_delete() {
    local file=$1
    local number=0
    number=$(tail -n 4 "${file}" | wc -l)
    if [ "${number}" -eq 0 ]; then
        return 1
    fi
    return 0
}

echo "--- Cleaning up GCP resources..."
echo "Validating configuration"
docker run -v $(pwd)/.buildkite/configs/cleanup.gcp.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_GCP_KEY_SECRET}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_GCP_EMAIL_SECRET}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_GCP_PROJECT_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    validate

echo "Scanning resources"
docker run -v $(pwd)/.buildkite/configs/cleanup.gcp.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_GCP_KEY_SECRET}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_GCP_EMAIL_SECRET}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_GCP_PROJECT_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    plan | tee "${GCP_RESOURCES_FILE}"

if any_resources_to_delete "${GCP_RESOURCES_FILE}"; then
    resources_to_delete=1
fi

echo "--- Cleaning up AWS resources..."
echo "Validating configuration"
docker run -v $(pwd)/.buildkite/configs/cleanup.aws.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_AWS_SECRET_KEY}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_AWS_USER_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    validate

echo "Scanning resources"
docker run -v $(pwd)/.buildkite/configs/cleanup.aws.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_AWS_SECRET_KEY}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_AWS_USER_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    plan | tee "${AWS_RESOURCES_FILE}"

if ! any_resources_to_delete "${AWS_RESOURCES_FILE}" ; then
    resource_to_delete=1
fi

if [ "${resource_to_delete}" -eq 1 ]; then
    message="There are resources to be deleted"
    if running_on_buildkite ; then
         buildkite-agent annotate \
             "There are resources to be deleted" \
             --style "error"
    fi
    echo "There are resources to be deleted"
    exit 1
fi

echo "--- Cleaning up other AWS resources"
echo "--- Installing awscli"
with_aws_cli

export AWS_ACCESS_KEY_ID="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_SECRET_ACCESS_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_DEFAULT_REGION=us-east-1

echo "--- Cleaning up Redshift resources"
