#!/bin/bash
set -euo pipefail

cleanup() {
    echo "Deleting temporal files..."
    cd ${WORKSPACE}
    rm -rf "tmp.elastic-package.*"
    echo "Done."
}
trap cleanup EXIT

usage() {
    echo "$0 [-t <target>] [-h]"
    echo "Trigger integration tests related to a target in Makefile"
    echo -e "\t-t <target>: Makefile target. Mandatory"
    echo -e "\t-p <package>: Package (required for test-check-packages-parallel target)."
    echo -e "\t-h: Show this message"
}

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

WORKSPACE="$(pwd)"
PARALLEL_TARGET="test-check-packages-parallel"
KIND_TARGET="test-check-packages-with-kind"
TEMPLATE_TEMP_FOLDER="tmp.elastic-package.XXXXXXXXX"
GOOGLE_CREDENTIALS_FILENAME="google-cloud-credentials.json"

JOB_GCS_BUCKET_INTERNAL="fleet-ci-temp-internal"

REPO_NAME=$(repoName "${BUILDKITE_REPO}")
REPO_BUILD_TAG="${REPO_NAME}/${BUILDKITE_BUILD_NUMBER}"

TARGET=""
PACKAGE=""
while getopts ":t:p:h" o; do
    case "${o}" in
        t)
            TARGET=${OPTARG}
            ;;
        p)
            PACKAGE=${OPTARG}
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

google_cloud_auth_safe_logs() {
    local gsUtilLocation=$(mktemp -d -p . -t ${TEMPLATE_TEMP_FOLDER})
    local secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}

    echo "${PRIVATE_CI_GCS_CREDENTIALS_SECRET}" > ${secretFileLocation}

    google_cloud_auth "${secretFileLocation}"

    echo "${gsUtilLocation}"
}

upload_safe_logs() {
    local bucket="$1"
    local source="$2"
    local target="$3"

    local gsUtilLocation=$(google_cloud_auth_safe_logs)

    gsutil cp "${source}" "gs://${bucket}/buildkite/${REPO_BUILD_TAG}/${target}"

    rm -rf "${gsUtilLocation}"
    unset GOOGLE_APPLICATIONS_CREDENTIALS
}

add_bin_path

echo "--- install go"
with_go

echo "--- install docker-compose"
with_docker_compose

if [[ "${TARGET}" == "${KIND_TARGET}" ]]; then
    echo "--- install kubectl & kind"
    with_kubernetes
fi

echo "--- Run integration test ${TARGET}"
if [[ "${TARGET}" == "${PARALLEL_TARGET}" ]]; then
    make install
    make PACKAGE_UNDER_TEST=${PACKAGE} ${TARGET}

    if [[ "${UPLOAD_SAFE_LOGS}" -eq 1 ]] ; then
        upload_safe_logs \
            "${JOB_GCS_BUCKET_INTERNAL}" \
            "build/elastic-stack-dump/check-${PACKAGE}/logs/elastic-agent-internal/*" \
            "insecure-logs/${PACKAGE}/"

        upload_safe_logs \
            "${JOB_GCS_BUCKET_INTERNAL}" \
            "build/container-logs/*.log" \
            "insecure-logs/${PACKAGE}/"
    fi
    exit 0
fi

make install ${TARGET} check-git-clean
