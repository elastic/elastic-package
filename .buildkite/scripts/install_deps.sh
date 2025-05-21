#!/bin/bash

source .buildkite/scripts/tooling.sh

set -euo pipefail

platform_type="$(uname)"
hw_type="$(uname -m)"
platform_type_lowercase="$(echo "${platform_type}" | tr '[:upper:]' '[:lower:]')"

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

with_docker() {
    if ! command -v docker &> /dev/null ; then
        echo "Skip. Docker is not installed by default"
        return
    fi

    echo "Setting up the Docker environment..."
    echo "- Current docker client version:"
    docker version -f json  | jq -r '.Client.Version'
    echo "- Current docker server version:"
    docker version -f json  | jq -r '.Server.Version'

    if [[ "${DOCKER_VERSION:-"false"}" == "false" ]]; then
        echo "Skip docker installation"
        return
    fi
    local ubuntu_version
    local ubuntu_codename
    local architecture
    ubuntu_version="$(lsb_release -rs)" # 20.04
    ubuntu_codename="$(lsb_release -sc)" # focal
    architecture=$(dpkg --print-architecture)
    local debian_version="5:${DOCKER_VERSION}-1~ubuntu.${ubuntu_version}~${ubuntu_codename}"

    sudo sudo mkdir -p /etc/apt/keyrings
    if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    fi
    echo "deb [arch=${architecture} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${ubuntu_codename} stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo DEBIAN_FRONTEND=noninteractive apt-get install --allow-change-held-packages --allow-downgrades -y "docker-ce=${debian_version}"
    sudo DEBIAN_FRONTEND=noninteractive apt-get install --allow-change-held-packages --allow-downgrades -y "docker-ce-cli=${debian_version}"
    sudo systemctl start docker

    echo "- Installed docker client version:"
    docker version -f json  | jq -r '.Client.Version'
    echo "- Installed docker server version:"
    docker version -f json  | jq -r '.Server.Version'
}

with_docker_compose_plugin() {
    if [[ "${DOCKER_COMPOSE_VERSION:-"false"}" == "false" ]]; then
        echo "Skip docker compose installation (plugin)"
        return
    fi
    create_bin_folder
    check_platform_architecture

    local DOCKER_CONFIG="$HOME/.docker/cli-plugins"
    mkdir -p "$DOCKER_CONFIG"

    retry 5 curl -sSL -o "${DOCKER_CONFIG}/docker-compose" "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-${platform_type_lowercase}-${hw_type}"
    chmod +x "${DOCKER_CONFIG}/docker-compose"
    docker compose version
}

with_kubernetes() {
    create_bin_folder
    check_platform_architecture

    retry 5 curl -sSLo "${WORKSPACE}/bin/kind" "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${platform_type_lowercase}-${arch_type}"
    chmod +x "${WORKSPACE}/bin/kind"
    kind version
    which kind

    retry 5 curl -sSLo "${WORKSPACE}/bin/kubectl" "https://dl.k8s.io/release/${K8S_VERSION}/bin/${platform_type_lowercase}/${arch_type}/kubectl"
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

with_yq() {
    create_bin_folder
    check_platform_architecture
    local binary="yq_${platform_type_lowercase}_${arch_type}"
    local bin_folder="${WORKSPACE}/bin"

    retry 5 curl -sSL -o "${bin_folder}/yq.tar.gz" "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${binary}.tar.gz"

    tar -C "${bin_folder}" -xpf "${bin_folder}/yq.tar.gz" "./${binary}"

    mv "${bin_folder}/${binary}" "${bin_folder}/yq"
    chmod +x "${bin_folder}/yq"
    yq --version

    rm -rf "${bin_folder}/yq.tar.gz"
}

with_aws_cli() {
    check_platform_architecture

    if ! which aws; then
        curl -s "https://awscli.amazonaws.com/awscli-exe-${platform_type_lowercase}-${hw_type}.zip" -o "awscliv2.zip"
        unzip -q awscliv2.zip
        sudo ./aws/install
        rm -rf awscliv2.zip aws
        aws --version
        return
    fi
    echo "\"aws\" already installed"
}
