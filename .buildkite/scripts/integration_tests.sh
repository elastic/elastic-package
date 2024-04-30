#!/bin/bash

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

set -euo pipefail

ensure_logout() {
    local error_code=$?

    if [ $error_code != 0 ] ; then
        # if variable is defined run the logout
        if [ -n "${GOOGLE_APPLICATION_CREDENTIALS+x}" ]; then
             google_cloud_logout_active_account
        fi
    fi

    exit $error_code
}
trap ensure_logout EXIT

usage() {
    echo "$0 [-t <target>] [-h]"
    echo "Trigger integration tests related to a target in Makefile"
    echo -e "\t-t <target>: Makefile target. Mandatory"
    echo -e "\t-p <package>: Package (required for test-check-packages-parallel target)."
    echo -e "\t-h: Show this message"
}

PARALLEL_TARGET="test-check-packages-parallel"
FALSE_POSITIVES_TARGET="test-check-packages-false-positives"
KIND_TARGET="test-check-packages-with-kind"
SYSTEM_TEST_FLAGS_TARGET="test-system-test-flags"
GOOGLE_CREDENTIALS_FILENAME="google-cloud-credentials.json"
ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT=${ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT:-"false"}

REPO_NAME=$(repo_name "${BUILDKITE_REPO}")
export REPO_BUILD_TAG="${REPO_NAME}/$(buildkite_pr_branch_build_id)"
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

add_bin_path

if [[ "$SERVERLESS" == "false" ]]; then
    # If packages are tested with Serverless, these action are already performed
    # here: .buildkite/scripts/test_packages_with_serverless.sh
    echo "--- install go"
    with_go

    echo "--- install docker"
    with_docker

    echo "--- install docker-compose plugin"
    with_docker_compose_plugin
fi

if [[ "${TARGET}" == "${KIND_TARGET}" || "${TARGET}" == "${SYSTEM_TEST_FLAGS_TARGET}" ]]; then
    echo "--- install kubectl & kind"
    with_kubernetes
fi

label="${TARGET}"
if [ -n "${PACKAGE}" ]; then
    label="${label} - ${PACKAGE}"
fi
echo "--- Run integration test ${label}"
if [[ "${TARGET}" == "${PARALLEL_TARGET}" ]] || [[ "${TARGET}" == "${FALSE_POSITIVES_TARGET}" ]]; then
    make install

    # allow to fail this command, to be able to upload safe logs
    set +e
    make SERVERLESS="${SERVERLESS}" PACKAGE_UNDER_TEST="${PACKAGE}" "${TARGET}"
    testReturnCode=$?
    set -e

    retry_count=${BUILDKITE_RETRY_COUNT:-"0"}

    if [[ "${UPLOAD_SAFE_LOGS}" -eq 1 ]] ; then
        package_folder="${PACKAGE}"
        if [[ "${ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT}" != "false" ]]; then
            package_folder="${package_folder}-independent_agent"
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
    fi

    if [ $testReturnCode != 0 ]; then
        echo "make SERVERLESS=${SERVERLESS} PACKAGE_UNDER_TEST=${PACKAGE} ${TARGET} failed with ${testReturnCode}"
        exit ${testReturnCode}
    fi

    make check-git-clean
    exit 0
fi

make install "${TARGET}" check-git-clean
