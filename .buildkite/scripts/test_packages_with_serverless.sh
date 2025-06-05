#!/usr/bin/env bash

source .buildkite/scripts/tooling.sh
source .buildkite/scripts/install_deps.sh

set -euo pipefail

UPLOAD_SAFE_LOGS=${UPLOAD_SAFE_LOGS:-"0"}

SKIPPED_PACKAGES_FILE_PATH="${WORKSPACE}/skipped_packages.txt"
FAILED_PACKAGES_FILE_PATH="${WORKSPACE}/failed_packages.txt"

export SERVERLESS="true"
SERVERLESS_PROJECT=${SERVERLESS_PROJECT:-"observability"}

add_pr_comment() {
    local source_pr_number="$1"
    local buildkite_build="$2"

    add_github_comment \
        "${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}" \
        "${source_pr_number}" \
        "Triggered serverless pipeline: ${buildkite_build}"
}

echo "Running packages on Serverles project type: ${SERVERLESS_PROJECT}"
if running_on_buildkite; then
    SERVERLESS_PROJECT="$(buildkite-agent meta-data get SERVERLESS_PROJECT --default "${SERVERLESS_PROJECT:-"observability"}")"
    buildkite-agent annotate "Serverless Project: ${SERVERLESS_PROJECT}" --context "ctx-info-${SERVERLESS_PROJECT}" --style "info"
fi


add_bin_path

echo "--- Install go"
with_go

echo "--- Install docker"
with_docker

echo "--- Install docker-compose"
with_docker_compose_plugin

if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
    echo "--- Install gh cli"
    with_github_cli

    add_pr_comment "${BUILDKITE_PULL_REQUEST}" "${BUILDKITE_BUILD_URL}"
fi

echo "--- Install elastic-package"
# Required to start the Serverless Elastic stack
make install

prepare_serverless_stack

echo "Waiting time to avoid getaddrinfo ENOTFOUND errors if any..."
sleep 120
echo "Done."

list_packages() {
    find test/packages/parallel -maxdepth 1 -mindepth 1 -type d | xargs -I {} basename {} | sort
}

any_package_failing=0

for package in $(list_packages); do
    if ! .buildkite/scripts/integration_tests.sh -t test-check-packages-parallel -p "${package}" -s ; then
        echo "- ${package}" >> "${FAILED_PACKAGES_FILE_PATH}"
        any_package_failing=1
    fi
done

if running_on_buildkite ; then
    if [ -f "${SKIPPED_PACKAGES_FILE_PATH}" ]; then
        create_collapsed_annotation "Skipped packages in ${SERVERLESS_PROJECT}" "${SKIPPED_PACKAGES_FILE_PATH}" "info" "ctx-skipped-packages-${SERVERLESS_PROJECT}"
    fi

    if [ -f "${FAILED_PACKAGES_FILE_PATH}" ]; then
        create_collapsed_annotation "Failed packages in ${SERVERLESS_PROJECT}" "${FAILED_PACKAGES_FILE_PATH}" "error" "ctx-failed-packages-${SERVERLESS_PROJECT}"
    fi
fi

if [ $any_package_failing -eq 1 ] ; then
    echo "These packages have failed:"
    cat "${FAILED_PACKAGES_FILE_PATH}"
    exit 1
fi
