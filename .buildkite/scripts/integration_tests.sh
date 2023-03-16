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

source .buildkite/scripts/install_deps.sh

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
    make install
    make PACKAGE_UNDER_TEST=${PACKAGE} ${TARGET}
    exit 0
fi

make install ${TARGET} check-git-clean
