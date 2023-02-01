#!/bin/bash

set -euo pipefail

with_kubernetes() {
    # FIXME add retry logic
    mkdir -p ${WORKSPACE}/bin
    curl -sSLo ${WORKSPACE}/bin/kind "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64"
    chmod +x ${WORKSPACE}/bin/kind
    kind version
    which kind

    mkdir -p ${WORKSPACE}/bin
    curl -sSLo ${WORKSPACE}/bin/kubectl "https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl"
    chmod +x ${WORKSPACE}/bin/kubectl
    kubectl version --client
    which kubectl
}

with_go() {
    # FIXME add retry logic
    mkdir -p ${WORKSPACE}/bin
    curl -sL -o ${WORKSPACE}/bin/gvm "https://github.com/andrewkroh/gvm/releases/download/${SETUP_GVM_VERSION}/gvm-linux-amd64"
    chmod +x ${WORKSPACE}/bin/gvm
    eval "$(gvm $(cat .go-version))"
    go version
    which go
}

with_docker_compose() {
    # FIXME add retry logic
    mkdir -p ${WORKSPACE}/bin
    curl -SL -o ${WORKSPACE}/bin/docker-compose "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-linux-x86_64"
    chmod +x ${WORKSPACE}/bin/docker-compose
    docker-compose version
}


echo "Current path: $(pwd)"
WORKSPACE="$(pwd)"
export PATH="${WORKSPACE}/bin:${PATH}"
echo "Path: $PATH"

echo "--- install go"
with_go
export PATH="$(go env GOPATH)/bin:${PATH}"

echo "--- install docker-compose"
with_docker_compose

echo "--- Run integration test stack-command-default"
make install test-stack-command-default check-git-clean
