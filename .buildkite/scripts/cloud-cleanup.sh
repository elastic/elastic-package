#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

set -euo pipefail

AWS_RESOURCES_FILE="aws.resources.txt"
GCP_RESOURCES_FILE="gcp.resources.txt"

RESOURCE_RETENTION_PERIOD="${RESOURCE_RETENTION_PERIOD:-"24 hours"}"
export DELETE_RESOURCES_BEFORE_DATE=$(date -Is -d "${RESOURCE_RETENTION_PERIOD} ago")

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

cloud_reaper_aws() {
    echo "Validating configuration"
    docker run -v $(pwd)/.buildkite/configs/cleanup.aws.yml:/etc/cloud-reaper/config.yml \
      -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_AWS_SECRET_KEY}" \
      -e ACCOUNT_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}" \
      -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_AWS_USER_SECRET}" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
        --config /etc/cloud-reaper/config.yml \
        validate

    echo "Scanning resources"
    docker run -v $(pwd)/.buildkite/configs/cleanup.aws.yml:/etc/cloud-reaper/config.yml \
      -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_AWS_SECRET_KEY}" \
      -e ACCOUNT_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}" \
      -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_AWS_USER_SECRET}" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
        --config /etc/cloud-reaper/config.yml \
        plan | tee "${AWS_RESOURCES_FILE}"
}

cloud_reaper_gcp() {
    echo "Validating configuration"
    docker run -v $(pwd)/.buildkite/configs/cleanup.gcp.yml:/etc/cloud-reaper/config.yml \
      -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_GCP_KEY_SECRET}" \
      -e ACCOUNT_KEY="${ELASTIC_PACKAGE_GCP_EMAIL_SECRET}" \
      -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_GCP_PROJECT_SECRET}" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
        --config /etc/cloud-reaper/config.yml \
        validate

    echo "Scanning resources"
    docker run -v $(pwd)/.buildkite/configs/cleanup.gcp.yml:/etc/cloud-reaper/config.yml \
      -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_GCP_KEY_SECRET}" \
      -e ACCOUNT_KEY="${ELASTIC_PACKAGE_GCP_EMAIL_SECRET}" \
      -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_GCP_PROJECT_SECRET}" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
        --config /etc/cloud-reaper/config.yml \
        plan | tee "${GCP_RESOURCES_FILE}"
}

echo "--- Cleaning up GCP resources..."
cloud_reaper_gcp

if any_resources_to_delete "${GCP_RESOURCES_FILE}"; then
    resources_to_delete=1
fi

echo "--- Cleaning up AWS resources..."
cloud_reaper_aws

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

# TODO: List and delete the required resources using aws cli
echo "--- Cleaning up other AWS resources"
echo "--- Installing awscli"
with_aws_cli

export AWS_ACCESS_KEY_ID="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_SECRET_ACCESS_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_DEFAULT_REGION=us-east-1

echo "+++ Cleaning up Redshift clusters"
echo "+++ Cleaning up IAM roles"
echo "+++ Cleaning up IAM policies"
echo "+++ Cleaning up Schedulers"
