#!/bin/bash

set -euo pipefail

PARALLEL_TARGET="test-check-packages-parallel"
KIND_TARGET="test-check-packages-with-kind"

usage() {
    echo "$0 [-t <target>] [-h]"
    echo "Trigger integration tests related to a target in Makefile"
    echo -e "\t-t <target>: Makefile target. Mandatory"
    echo -e "\t-p <package>: Package (required for test-check-packages-parallel target)."
    echo -e "\t-h: Show this message"
}

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

TARGET=""
PACKAGE=""
while getopts ":t:p:h" o; do
    case "${o}" in
        t)
            TARGET=${OPTARG}
            ;;
        p)
            PACKAGE=${OPTARG}
            ;;
        h)
            usage
            exit 0
            ;;
        \?)
            echo "Invalid option ${OPTARG}"
            usage
            exit 1
            ;;
        :)
            echo "Missing argument for -${OPTARG}"
            usage
            exit 1
            ;;
    esac
done

if [[ "${TARGET}" == "" ]]; then
    echo "Missing target"
    usage
    exit 1
fi

echo "Current path: $(pwd)"
WORKSPACE="$(pwd)"
export PATH="${WORKSPACE}/bin:${PATH}"
echo "Path: $PATH"

echo "--- install go"
with_go
export PATH="$(go env GOPATH)/bin:${PATH}"

echo "--- install docker-compose"
with_docker_compose

if [[ "${TARGET}" == "${KIND_TARGET}" ]]; then
    echo "--- install kubectl & kind"
    with_kubernetes
fi

echo "--- Run integration test ${TARGET}"
if [[ "${TARGET}" == "${PARALLEL_TARGET}" ]]; then
    if [[ "${PACKAGE}" == "aws" ]]; then
        echo "Test dummy key: ${AWS_SECRET_ACCESS_KEY} - ${AWS_ACCESS_KEY_ID}"
        exit 0
    fi
    make install
    make PACKAGE_UNDER_TEST=${PACKAGE} ${TARGET}
    exit 0
fi

make install ${TARGET} check-git-clean
