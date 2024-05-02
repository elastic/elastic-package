#!/bin/bash
set -euo pipefail

unset_secrets () {
  for var in $(printenv | sed 's;=.*;;' | sort); do
    if [[ "$var" == *_SECRET || "$var" == *_TOKEN ]]; then
        unset "$var"
    fi
  done
}

repo_name() {
    # Example of URL: git@github.com:acme-inc/my-project.git
    local repoUrl=$1

    orgAndRepo=$(echo "$repoUrl" | cut -d':' -f 2)
    basename "${orgAndRepo}" .git
}

buildkite_pr_branch_build_id() {
    if [ "${BUILDKITE_PULL_REQUEST}" != "false" ]; then
        echo "PR-${BUILDKITE_PULL_REQUEST}-${BUILDKITE_BUILD_NUMBER}"
        return
    fi

    if [[ "${BUILDKITE_PIPELINE_SLUG}" == "elastic-package" ]]; then
        echo "${BUILDKITE_BRANCH}-${BUILDKITE_BUILD_NUMBER}"
        return
    fi
    # Other pipelines
    echo "${BUILDKITE_BRANCH}-${BUILDKITE_PIPELINE_SLUG}-${BUILDKITE_BUILD_NUMBER}"
}

google_cloud_auth() {
    local keyFile=$1

    gcloud auth activate-service-account --key-file "${keyFile}" 2> /dev/null

    export GOOGLE_APPLICATION_CREDENTIALS=${keyFile}
}

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

google_cloud_logout_active_account() {
  local active_account
  active_account=$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null || true)
  if [[ -n "$active_account" && -n "${GOOGLE_APPLICATION_CREDENTIALS+x}" ]]; then
    echo "Logging out from GCP for active account"
    gcloud auth revoke "$active_account" > /dev/null 2>&1
  else
    echo "No active GCP accounts found."
  fi
  if [ -n "${GOOGLE_APPLICATION_CREDENTIALS+x}" ]; then
    rm -rf "${GOOGLE_APPLICATION_CREDENTIALS}"
    unset GOOGLE_APPLICATION_CREDENTIALS
  fi
}

running_on_buildkite() {
    if [[ "${BUILDKITE:-"false"}" == "true" ]]; then
        return 0
    fi
    return 1
}

create_elastic_package_profile() {
    local name="$1"
    elastic-package profiles create "${name}"
    elastic-package profiles use "${name}"
}

prepare_serverless_stack() {
    echo "--- Prepare serverless stack"

    # Creating a new profile allow to set a specific name for the serverless project
    local profile_name="elastic-package-${BUILDKITE_PIPELINE_SLUG}-${BUILDKITE_BUILD_NUMBER}-${SERVERLESS_PROJECT}"
    if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
        profile_name="elastic-package-${BUILDKITE_PULL_REQUEST}-${BUILDKITE_BUILD_NUMBER}-${SERVERLESS_PROJECT}"
    fi
    create_elastic_package_profile "${profile_name}"

    export EC_API_KEY=${EC_API_KEY_SECRET}
    export EC_HOST=${EC_HOST_SECRET}

    echo "Boot up the Elastic stack"
    # Currently, if STACK_VERSION is not defined, for serverless it will be
    # used as Elastic stack version (for agents) the default version in elastic-package
    local stack_version=${STACK_VERSION:-""}
    local args="-v"
    if [ -n "${stack_version}" ]; then
        args="${args} --version ${stack_version}"
    fi

    # grep command required to remove password from the output
    if ! elastic-package stack up \
        -d \
        ${args} \
        --provider serverless \
        -U "stack.serverless.region=${EC_REGION_SECRET},stack.serverless.type=${SERVERLESS_PROJECT}" 2>&1 | grep -E -v "^Password: " ; then
        return 1
    fi
    echo ""
    elastic-package stack status
    echo ""
}

google_cloud_auth_safe_logs() {
    local gsUtilLocation=""
    gsUtilLocation=$(mktemp -d -p "${WORKSPACE}" -t "${TMP_FOLDER_TEMPLATE}")

    local secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}

    echo "${PRIVATE_CI_GCS_CREDENTIALS_SECRET}" > "${secretFileLocation}"

    google_cloud_auth "${secretFileLocation}"
}

upload_safe_logs() {
    local bucket="$1"
    local source="$2"
    local target="$3"

    if ! ls ${source} 2>&1 > /dev/null ; then
        echo "upload_safe_logs: artifacts files not found, nothing will be archived"
        return
    fi

    google_cloud_auth_safe_logs

    gsutil cp ${source} "gs://${bucket}/buildkite/${REPO_BUILD_TAG}/${target}"

    google_cloud_logout_active_account
}

clean_safe_logs() {
    rm -rf "${WORKSPACE}/build/elastic-stack-dump"
    rm -rf "${WORKSPACE}/build/container-logs"
}

cleanup() {
  echo "Deleting temporary files..."
  rm -rf ${WORKSPACE}/${TMP_FOLDER_TEMPLATE_BASE}.*
  echo "Done."
}

create_collapsed_annotation() {
    local title="$1"
    local file="$2"
    local style="$3"
    local context="$4"

    local annotation_file="tmp.annotation.md"
    echo "<details><summary>${title}</summary>" >> ${annotation_file}
    echo -e "\n\n" >> ${annotation_file}
    cat "${file}" >> ${annotation_file}
    echo "</details>" >> ${annotation_file}

    cat ${annotation_file} | buildkite-agent annotate --style "${style}" --context "${context}"

    rm -f ${annotation_file}
}

add_github_comment() {
    local repository="$1"
    local pr_id="$2"
    local message="$3"

    retry 3 \
        gh pr comment "${pr_id}" \
        --body "${message}" \
        --repo "${repository}"
}
