#!/bin/bash

source .buildkite/scripts/tooling.sh

set -euo pipefail

create_bin_folder() {
    mkdir -p "${WORKSPACE}/bin"
}

add_bin_path(){
    create_bin_folder
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_kubernetes() {
    create_bin_folder
    retry 5 curl -sSLo "${WORKSPACE}/bin/kind" "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64"
    chmod +x "${WORKSPACE}/bin/kind"
    kind version
    which kind

    retry 5 curl -sSLo "${WORKSPACE}/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl"
    chmod +x "${WORKSPACE}/bin/kubectl"
    kubectl version --client
    which kubectl
}

with_go() {
    create_bin_folder
    retry 5 curl -sL -o "${WORKSPACE}/bin/gvm" "https://github.com/andrewkroh/gvm/releases/download/${SETUP_GVM_VERSION}/gvm-linux-amd64"
    chmod +x "${WORKSPACE}/bin/gvm"
    eval "$(gvm "$(cat .go-version)")"
    go version
    which go
    PATH="${PATH}:$(go env GOPATH)/bin"
    export PATH
}

with_github_cli() {
    create_bin_folder
    mkdir -p "${WORKSPACE}/tmp"

    local gh_filename="gh_${GH_CLI_VERSION}_linux_amd64"
    local gh_tar_file="${gh_filename}.tar.gz"
    local gh_tar_full_path="${WORKSPACE}/tmp/${gh_tar_file}"

    retry 5 curl -sL -o "${gh_tar_full_path}" "https://github.com/cli/cli/releases/download/v${GH_CLI_VERSION}/${gh_tar_file}"

    # just extract the binary file from the tar.gz
    tar -C "${WORKSPACE}/bin" -xpf "${gh_tar_full_path}" "${gh_filename}/bin/gh" --strip-components=2

    chmod +x "${WORKSPACE}/bin/gh"
    rm -rf "${WORKSPACE}/tmp"

    gh version
}

with_jq() {
    create_bin_folder
    retry 5 curl -sL -o "${WORKSPACE}/bin/jq" "https://github.com/stedolan/jq/releases/download/jq-${JQ_VERSION}/jq-linux64"

    chmod +x "${WORKSPACE}/bin/jq"
    jq --version
}
