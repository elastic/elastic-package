#!/bin/bash

source .buildkite/scripts/tooling.sh

set -euo pipefail

platform_type="$(uname)"
hw_type="$(uname -m)"
platform_type_lowercase="${platform_type,,}"

check_platform_architecture() {
  case "${hw_type}" in
    "x86_64")
      arch_type="amd64"
      ;;
    "aarch64")
      arch_type="arm64"
      ;;
    "arm64")
      arch_type="arm64"
      ;;
    *)
    echo "The current platform/OS type is unsupported yet"
    ;;
  esac
}

create_bin_folder() {
    mkdir -p "${WORKSPACE}/bin"
}

add_bin_path(){
    create_bin_folder
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_kubernetes() {
    create_bin_folder
    check_platform_architecture

    retry 5 curl -sSLo "${WORKSPACE}/bin/kind" "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${platform_type_lowercase}-${arch_type}"
    chmod +x "${WORKSPACE}/bin/kind"
    kind version
    which kind

    retry 5 curl -sSLo "${WORKSPACE}/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${platform_type_lowercase}/${arch_type}/kubectl"
    chmod +x "${WORKSPACE}/bin/kubectl"
    kubectl version --client
    which kubectl
}

with_go() {
    create_bin_folder
    check_platform_architecture

    echo "GVM ${SETUP_GVM_VERSION} (platform ${platform_type_lowercase} arch ${arch_type}"
    retry 5 curl -sL -o "${WORKSPACE}/bin/gvm" "https://github.com/andrewkroh/gvm/releases/download/${SETUP_GVM_VERSION}/gvm-${platform_type_lowercase}-${arch_type}"

    chmod +x "${WORKSPACE}/bin/gvm"
    eval "$(gvm "$(cat .go-version)")"
    go version
    which go
    PATH="${PATH}:$(go env GOPATH)/bin"
    export PATH
}

with_github_cli() {
    create_bin_folder
    check_platform_architecture

    mkdir -p "${WORKSPACE}/tmp"

    local gh_filename="gh_${GH_CLI_VERSION}_${platform_type_lowercase}_${arch_type}"
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
    check_platform_architecture
    # filename for versions <=1.6 is jq-linux64
    local binary="jq-${platform_type_lowercase}-${arch_type}"

    retry 5 curl -sL -o "${WORKSPACE}/bin/jq" "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/${binary}"

    chmod +x "${WORKSPACE}/bin/jq"
    jq --version
}
