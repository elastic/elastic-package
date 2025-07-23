#!/bin/bash
source .buildkite/scripts/install_deps.sh

set -euo pipefail

add_bin_path

echo "--- install gh cli"
with_github_cli

echo "--- install jq"
with_jq


INTEGRATIONS_SOURCE_BRANCH=main
INTEGRATIONS_GITHUB_OWNER=elastic
INTEGRATIONS_GITHUB_REPO_NAME=integrations
INTEGRATIONS_PR_BRANCH="test-${GITHUB_PR_BASE_REPO}-pr-${BUILDKITE_PULL_REQUEST}"
INTEGRATIONS_PR_TITLE="Test ${GITHUB_PR_BASE_REPO}#${BUILDKITE_PULL_REQUEST} - DO NOT MERGE"
VERSION_DEP=""

get_pr_number() {
    # requires GITHUB_TOKEN
    local branch="$1"
    gh pr list -H "${branch}" --json number | jq -r '.[]|.number'
}

get_integrations_pr_link() {
    local pr_number=$1
    echo "https://github.com/elastic/integrations/pull/${pr_number}"
}

get_source_pr_link() {
    echo "https://github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}/pull/${BUILDKITE_PULL_REQUEST}"
}

get_source_commit_link() {
    echo "https://github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}/commit/${GITHUB_PR_HEAD_SHA}"
}

set_git_config() {
    git config user.name "${GITHUB_USERNAME}"
    git config user.email "${GITHUB_EMAIL}"
}

git_push() {
    local branch="$1"

    retry 3 git push origin "${branch}"
}

clone_repository() {
    local target="$1"
    retry 5 git clone https://github.com/elastic/integrations "${target}"
}

# This function retrieves the assignees for the pull request.
# It checks if the head user is not "elastic" or the trigger user, and adds it to the list of assignees.
# It returns a comma-separated list of assignees.
get_assignees() {
    local assignees="${GITHUB_PR_TRIGGER_USER}"
    if [[ "${GITHUB_PR_HEAD_USER}" != "elastic" && "${GITHUB_PR_HEAD_USER}" != "${GITHUB_PR_TRIGGER_USER}" ]]; then
        assignees="${assignees},${GITHUB_PR_HEAD_USER}"
    fi
    echo "${assignees}"
}

create_integrations_pull_request() {
    local assignees=""
    assignees=$(get_assignees)

    # requires GITHUB_TOKEN
    local temp_path
    temp_path=$(mktemp -d -p "${WORKSPACE}" -t "${TMP_FOLDER_TEMPLATE}")
    echo "Creating Pull Request"
    message="Update ${GITHUB_PR_BASE_REPO} reference to $(get_source_commit_link).\nAutomated by [Buildkite build](${BUILDKITE_BUILD_URL})\n\nRelates: $(get_source_pr_link)"
    echo -e "$message" > "${temp_path}/body-pr.txt"
    retry 3 \
        gh pr create \
        --title "${INTEGRATIONS_PR_TITLE}" \
        --body-file "${temp_path}/body-pr.txt" \
        --draft \
        --base "${INTEGRATIONS_SOURCE_BRANCH}" \
        --head "${INTEGRATIONS_PR_BRANCH}" \
        --assignee "${assignees}"
}

update_dependency() {
    # it needs to set the Golang version from the integrations repository (.go-version file)
    echo "--- install go for integrations repository :go:"
    with_go

    echo "--- Updating go.mod and go.sum with ${GITHUB_PR_HEAD_SHA} :hammer_and_wrench:"
    local source_dep="github.com/${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}${VERSION_DEP}"
    local target_dep="github.com/${GITHUB_PR_OWNER}/${GITHUB_PR_REPO}${VERSION_DEP}@${GITHUB_PR_HEAD_SHA}"

    go mod edit -replace "${source_dep}=${target_dep}"
    go mod tidy

    git add go.mod
    git add go.sum

    # allow not to commit if there are no changes
    # previous execution could fail and just pushed the branch but PR is not created
    if ! git diff-index --quiet HEAD ; then
        git commit -m "Test elastic-package from PR ${BUILDKITE_PULL_REQUEST} - ${GITHUB_PR_HEAD_SHA}"
    fi

    echo ""
    git --no-pager show --format=oneline HEAD
    echo ""
}

exists_branch() {
    local owner="$1"
    local repository="$2"
    local branch="$3"

    git ls-remote --exit-code --heads "https://github.com/${owner}/${repository}.git" "${branch}"
}

create_or_update_pull_request() {
    local temp_path
    temp_path=$(mktemp -d -p "${WORKSPACE}" -t "${TMP_FOLDER_TEMPLATE}")
    local repo_path="${temp_path}/elastic-integrations"
    local checkout_options=""
    local integrations_pr_number=""

    echo "Cloning repository"
    clone_repository "${repo_path}"

    pushd "${repo_path}" > /dev/null

    set_git_config

    echo "Checking branch ${INTEGRATIONS_PR_BRANCH} in remote ${INTEGRATIONS_GITHUB_OWNER}/${INTEGRATIONS_GITHUB_REPO_NAME}"
    if ! exists_branch "${INTEGRATIONS_GITHUB_OWNER}" "${INTEGRATIONS_GITHUB_REPO_NAME}" "${INTEGRATIONS_PR_BRANCH}" ; then
        checkout_options=" -b "
        echo "Creating a new branch..."
    else
        echo "Already existed"
    fi

    integrations_pr_number=$(get_pr_number "${INTEGRATIONS_PR_BRANCH}")
    if [ -z "${integrations_pr_number}" ]; then
        echo "Exists PR in integrations repository: ${integrations_pr_number}"
    fi

    git checkout ${checkout_options} "${INTEGRATIONS_PR_BRANCH}"

    echo "--- Updating dependency :pushpin:"
    update_dependency

    echo "--- Pushing branch ${INTEGRATIONS_PR_BRANCH} to integrations repository..."
    git_push "${INTEGRATIONS_PR_BRANCH}"

    if [ -z "${integrations_pr_number}" ]; then
        echo "--- Creating pull request :github:"
        create_integrations_pull_request

        sleep 10

        integrations_pr_number=$(get_pr_number "${INTEGRATIONS_PR_BRANCH}")
    fi

    popd > /dev/null

    rm -rf "${temp_path}"

    echo "--- adding comment into ${GITHUB_PR_BASE_REPO} pull request :memo:"
    add_pr_comment "${BUILDKITE_PULL_REQUEST}" "$(get_integrations_pr_link "${integrations_pr_number}")"
}

add_pr_comment() {
    local source_pr_number="$1"
    local integrations_pr_link="$2"

    add_github_comment \
        "${GITHUB_PR_BASE_OWNER}/${GITHUB_PR_BASE_REPO}" \
        "${source_pr_number}" \
        "Created or updated PR in integrations repository to test this version. Check ${integrations_pr_link}"
}

if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
    echo "Currently, this pipeline is just supported in Pull Requests."
    exit 1
fi

echo "--- creating or updating integrations pull request"
create_or_update_pull_request
