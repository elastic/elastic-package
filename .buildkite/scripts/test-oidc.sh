#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

set -euo pipefail
PARALLEL_TARGET="test-check-packages-parallel"
FALSE_POSITIVES_TARGET="test-check-packages-false-positives"
KIND_TARGET="test-check-packages-with-kind"
SYSTEM_TEST_FLAGS_TARGET="test-system-test-flags"
TEST_BUILD_ZIP_TARGET="test-build-zip"
export JOB_GCS_BUCKET_INTERNAL="ecosystem-ci-internal"
REPO_NAME=$(repo_name "${BUILDKITE_REPO}")
export REPO_BUILD_TAG="${REPO_NAME}/$(buildkite_pr_branch_build_id)"
mkdir -p build/elastic-stack-dump
touch build/elastic-stack-dump/elastic-agent-internal0
touch build/elastic-stack-dump/elastic-agent-internal1

upload_safe_logs \
    "${JOB_GCS_BUCKET_INTERNAL}" \
    "build/elastic-stack-dump/*.*" \
    "insecure-logs/test/elastic-agent-logs/"