#!/bin/bash

set -euo pipefail

source .buildkite/scripts/tooling.sh

add_bin_path(){
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_kubernetes() {
    mkdir -p ${WORKSPACE}/bin
    retry 5 curl -sSLo ${WORKSPACE}/bin/kind "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64"
    chmod +x ${WORKSPACE}/bin/kind
    kind version
    which kind

    mkdir -p ${WORKSPACE}/bin
    retry 5 curl -sSLo ${WORKSPACE}/bin/kubectl "https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl"
    chmod +x ${WORKSPACE}/bin/kubectl
    kubectl version --client
    which kubectl
}

with_go() {
    mkdir -p ${WORKSPACE}/bin
    retry 5 curl -sL -o ${WORKSPACE}/bin/gvm "https://github.com/andrewkroh/gvm/releases/download/${SETUP_GVM_VERSION}/gvm-linux-amd64"
    chmod +x ${WORKSPACE}/bin/gvm
    eval "$(gvm $(cat .go-version))"
    go version
    which go
    export PATH="$(go env GOPATH)/bin:${PATH}"
}

with_docker_compose() {
    mkdir -p ${WORKSPACE}/bin
    retry 5 curl -SL -o ${WORKSPACE}/bin/docker-compose "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-linux-x86_64"
    chmod +x ${WORKSPACE}/bin/docker-compose
    docker-compose version
}
