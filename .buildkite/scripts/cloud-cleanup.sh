#!/usr/bin/env bash

set -euo pipefail

# TODO: change to 24 hours ago
export CREATION_DATE=$(date -Is -d "1 minute ago")

echo "--- Cleaning up AWS resources..."
docker run -v $(pwd)/.buildkite/configs/cleanup.aws.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_AWS_SECRET_KEY}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_AWS_USER_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    plan

echo "--- Cleaning up GCP resources..."
docker run -v $(pwd)/.buildkite/configs/cleanup.gcp.yml:/etc/cloud-reaper/config.yml \
  -e ACCOUNT_SECRET="${ELASTIC_PACKAGE_GCP_KEY_SECRET}" \
  -e ACCOUNT_KEY="${ELASTIC_PACKAGE_GCP_EMAIL_SECRET}" \
  -e ACCOUNT_PROJECT="${ELASTIC_PACKAGE_GCP_PROJECT_SECRET}" \
  -e CREATION_DATE=${CREATION_DATE} \
  ${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0 cloud-reaper \
    --config /etc/cloud-reaper/config.yml \
    plan
