#!/bin/bash

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

set -euo pipefail

usage() {
    echo "$0 [-t <target>] [-h]"
    echo "Trigger integration tests related to a target in Makefile"
    echo -e "\t-t <target>: Makefile target. Mandatory"
    echo -e "\t-p <package>: Package (required for test-check-packages-parallel target)."
    echo -e "\t-h: Show this message"
}

export UPLOAD_SAFE_LOGS=${UPLOAD_SAFE_LOGS:-"0"}

PARALLEL_TARGET="test-check-packages-parallel"
FALSE_POSITIVES_TARGET="test-check-packages-false-positives"
KIND_TARGET="test-check-packages-with-kind"
SYSTEM_TEST_FLAGS_TARGET="test-system-test-flags"
TEST_BUILD_ZIP_TARGET="test-build-zip"
TEST_BUILD_INSTALL_ZIP_TARGET="test-build-install-zip"

REPO_NAME=$(repo_name "${BUILDKITE_REPO}")
REPO_BUILD_TAG="${REPO_NAME}/$(buildkite_pr_branch_build_id)"
export REPO_BUILD_TAG
TARGET=""
PACKAGE=""
SERVERLESS="false"
while getopts ":t:p:sh" o; do
    case "${o}" in
        t)
            TARGET=${OPTARG}
            ;;
        p)
            PACKAGE=${OPTARG}
            ;;
        s)
            SERVERLESS="true"
            ;;
        h)
            usage
            exit 0
            ;;
        \?)
            echo "Invalid option ${OPTARG}"
            usage
            exit 1
            ;;
        :)
            echo "Missing argument for -${OPTARG}"
            usage
            exit 1
            ;;
    esac
done

if [[ "${TARGET}" == "" ]]; then
    echo "Missing target"
    usage
    exit 1
fi

upload_package_test_logs() {
    local retry_count=0
    local package_folder=""

    if [[ "${PACKAGE}" == "" ]]; then
        echo "No package specified, skipping upload of safe logs"
        return
    fi

    echo "--- Uploading safe logs to GCP bucket ${JOB_GCS_BUCKET_INTERNAL}"

    retry_count=${BUILDKITE_RETRY_COUNT:-"0"}
    # Add target as part of the package folder name to allow to distinguish
    # different test runs for the same package in different Makefile targets.
    # Currently, just for test-check-packages-* targets, but could be extended
    # to other targets in the future.
    target=${TARGET#"test-check-packages-"}
    package_folder="${target}.${PACKAGE}"

    if [[ "${ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT:-""}" == "false" ]]; then
        package_folder="${package_folder}-stack_agent"
    fi

    if [[ "${ELASTIC_PACKAGE_FIELD_VALIDATION_TEST_METHOD:-""}" != "" ]]; then
        package_folder="${package_folder}-${ELASTIC_PACKAGE_FIELD_VALIDATION_TEST_METHOD}"
    fi

    if [[ "${retry_count}" -ne 0 ]]; then
        package_folder="${package_folder}_retry_${retry_count}"
    fi

    upload_safe_logs \
        "${JOB_GCS_BUCKET_INTERNAL}" \
        "build/elastic-stack-dump/check-${PACKAGE}/logs/elastic-agent-internal/*.*" \
        "insecure-logs/${package_folder}/elastic-agent-logs/"

    # required for <8.6.0
    upload_safe_logs \
        "${JOB_GCS_BUCKET_INTERNAL}" \
        "build/elastic-stack-dump/check-${PACKAGE}/logs/elastic-agent-internal/default/*" \
        "insecure-logs/${package_folder}/elastic-agent-logs/default/"

    upload_safe_logs \
        "${JOB_GCS_BUCKET_INTERNAL}" \
        "build/container-logs/*.log" \
        "insecure-logs/${package_folder}/container-logs/"
}

install_required_tools() {
    local target="${1}"

    if [[ "${SERVERLESS}" == "true" ]]; then
        # If packages are tested with Serverless, these action are already performed
        # here: .buildkite/scripts/test_packages_with_serverless.sh
        echo "Skipping installation of required tools for Serverless testing"
        return
    fi

    add_bin_path

    echo "--- Install go"
    with_go

    if [[ "${target}" != "${TEST_BUILD_ZIP_TARGET}" ]]; then
        # Not supported in Macos ARM
        echo "--- Install docker"
        with_docker

        echo "--- Install docker-compose plugin"
        with_docker_compose_plugin
    fi

    case "${target}" in
        "${KIND_TARGET}" | "${SYSTEM_TEST_FLAGS_TARGET}")
            echo "--- Install kind"
            with_kubernetes
            ;;
        "${FALSE_POSITIVES_TARGET}" | "${TEST_BUILD_INSTALL_ZIP_TARGET}")
            echo "--- Install yq"
            with_yq
            ;;
    esac

    # In Serverless pipeline, elastic-package is installed in advance here:
    # .buildkite/scripts/test_packages_with_serverless.sh
    # No need to install it again for every package.
    echo "--- Install elastic-package"
    make install
}

if [[ "${SERVERLESS}" == "true" && "${TARGET}" != "${PARALLEL_TARGET}" ]]; then
    # Just tested parallel target to run with Serverless projects, other Makefile targets
    # have not been tested yet and could fail unexpectedly. For instance, "test-check-packages-false-positives"
    # target would require a different management to not stop Elastic stack after each package test.
    echo "Target ${TARGET} is not supported for Serverless testing"
    usage
    exit 1
fi

install_required_tools "${TARGET}"

label="${TARGET}"
if [ -n "${PACKAGE}" ]; then
    label="${label} - ${PACKAGE}"
fi

echo "--- Run integration test ${label}"
# allow to fail this command, to be able to upload safe logs
set +e
make SERVERLESS="${SERVERLESS}" PACKAGE_UNDER_TEST="${PACKAGE}" "${TARGET}"
testReturnCode=$?
set -e

if [[ "${UPLOAD_SAFE_LOGS}" -eq 1 ]] ; then
    upload_package_test_logs
fi

if [ $testReturnCode != 0 ]; then
    echo "make SERVERLESS=${SERVERLESS} PACKAGE_UNDER_TEST=${PACKAGE} ${TARGET} failed with ${testReturnCode}"
    exit ${testReturnCode}
fi

echo "--- Check git clean"
make check-git-clean
