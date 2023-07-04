#!/bin/bash
set -euo pipefail

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

    export GOOGLE_APPLICATIONS_CREDENTIALS=${secretFileLocation}
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

