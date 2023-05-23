#!/bin/bash

set -euo pipefail

WORKSPACE="$(pwd)"

TMP_FOLDER_TEMPLATE="tmp.repo"

cleanup() {
    echo "Deleting temporal files..."
    cd ${WORKSPACE}
    rm -rf "${TMP_FOLDER_TEMPLATE}.*"
    echo "Done."
}

trap cleanup EXIT
source .buildkite/scripts/install_deps.sh

add_bin_path

echo "--- install gh cli"
with_github_cli

echo "--- install jq"
with_jq

INTEGRATIONS_SOURCE_BRANCH=main
INTEGRATIONS_REPO=github.com:elastic/integrations.git
INTEGRATIONS_PR_BRANCH="test-elastic-package-pr-${BUILDKITE_PULL_REQUEST}"
INTEGRATIONS_PR_TITLE="Test elastic-package - DO NOT MERGE"


get_pr_number() {
    local branch="$1"
    gh pr list -H "${branch}" --json number | jq -r '.[]|.number'
}

get_integrations_pr_link() {
    local pr_number=$1
    echo "https://github.com/elastic/integrations/pull/${pr_number}"
}

get_elastic_package_pr_link() {
    echo "https://github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}/pull/${BUILDKITE_PULL_REQUEST}"
}

set_git_config() {
    git config user.name "${GITHUB_USERNAME_SECRET}"
    git config user.email "${GITHUB_EMAIL_SECRET}"
}

git_push_with_auth() {
    local branch=$1

    retry 3 git push https://${GITHUB_USERNAME_SECRET}:${GITHUB_TOKEN}@github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}.git "${branch}"
}

clone_repository() {
    local target="$1"
    retry 5 git clone https://github.com/elastic/integrations ${target}
}

create_pull_request() {
    echo "Creating Pull Request"
    retry 3 \
        gh pr create \
        --title "${INTEGRATIONS_PR_TITLE}" \
        --body "Update elastic-package reference to ${GITHUB_PR_HEAD_SHA}.\nAutomated by [Buildkite build](${BUILDKITE_BUILD_URL})\n\nRelates: $(get_elastic_package_pr_link)" \
        --draft \
        --base ${INTEGRATIONS_SOURCE_BRANCH} \
        --head ${INTEGRATIONS_PR_BRANCH} \
        --assignee ${BUILDKITE_PR_HEAD_USER}
}

update_dependency() {
    go mod edit -replace github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}=github.com/${GITHUB_PR_OWNER}/${GITHUB_PR_REPO}@${GITHUB_PR_HEAD_SHA}
    go mod tidy

    git add go.mod
    git add go.sum

    git commit -m "Test elastic-package from PR ${BUILDKITE_PULL_REQUEST} - ${GITHUB_PR_HEAD_SHA}"
}


create_or_update_pull_request() {
    local temp_path=$(mktemp -d -p ${WORKSPACE} -t ${TMP_FOLDER_TEMPLATE})
    local repo_path="${temp_path}/elastic-integrations"
    local checkout_options=""
    local integrations_pr_number=""

    clone_repository "${repo_path}"
    pushd "${repo_path}" > /dev/null

    set_git_config

    integrations_pr_number=$(get_pr_number "${INTEGRATIONS_PR_BRANCH}")
    if [ -z "${integrations_pr_number}" ]; then
        checkout_options=" -b "
    fi
    git checkout ${checkout_options} ${INTEGRATIONS_PR_BRANCH}

    update_dependency

    return
    git_push_with_auth

    if [ -z "${integrations_pr_number}" ]; then
        create_pull_request
    fi

    popd > /dev/null

    rm -rf "${temp_path}"
}


add_pr_comment() {
    local pr_number="$1"
    retry 3 \
        gh pr comment ${pr_number} \
        --body "Created or updated PR in integrations repostiory to test this vesrion. Check ${get_integrations_pr_link}" \
        --repo ${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}
}


echo "--- creating or updating integrations pull request"
create_or_update_pull_request

exit 0

echo "--- adding comment into elastic-package pull request"
add_pr_comment "${BUILDKITE_PULL_REQUEST}"
