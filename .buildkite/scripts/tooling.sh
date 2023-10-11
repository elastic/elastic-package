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

    orgAndRepo=$(echo $repoUrl | cut -d':' -f 2)
    echo "$(basename ${orgAndRepo} .git)"
}

buildkite_pr_branch_build_id() {
    if [ "${BUILDKITE_PULL_REQUEST}" == "false" ]; then
        echo "${BUILDKITE_BRANCH}-${BUILDKITE_BUILD_NUMBER}"
        return
    fi
    echo "PR-${BUILDKITE_PULL_REQUEST}-${BUILDKITE_BUILD_NUMBER}"
}

google_cloud_auth() {
    local keyFile=$1

    gcloud auth activate-service-account --key-file ${keyFile} 2> /dev/null

    export GOOGLE_APPLICATION_CREDENTIALS=${secretFileLocation}
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
  local active_account=$(gcloud auth list --filter=status:ACTIVE --format="value(account)" 2>/dev/null)
  if [[ -n "$active_account" && -n "${GOOGLE_APPLICATION_CREDENTIALS+x}" ]]; then
    echo "Logging out from GCP for active account"
    gcloud auth revoke $active_account > /dev/null 2>&1
  else
    echo "No active GCP accounts found."
  fi
  if [ -n ${GOOGLE_APPLICATION_CREDENTIALS+x} ]; then
    rm -rf ${GOOGLE_APPLICATION_CREDENTIALS}
    unset GOOGLE_APPLICATION_CREDENTIALS
  fi
}
