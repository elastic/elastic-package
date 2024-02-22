#!/usr/bin/env bash

set -euo pipefail

# TODO: change to 24 hours ago
export CREATION_DATE=$(date -Is -d "1 minute ago")

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
    plan

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
    plan

echo "--- Cleaning up other AWS resources"
echo "--- Installing awscli"
if ! which aws; then
  curl -s "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
  unzip -q awscliv2.zip
  sudo ./aws/install
  rm -rf awscliv2.zip aws
  aws --version
fi

export AWS_ACCESS_KEY_ID="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_SECRET_ACCESS_KEY="${ELASTIC_PACKAGE_AWS_ACCESS_KEY}"
export AWS_DEFAULT_REGION=us-east-1

echo "--- Cleaning up Redshift resources"
