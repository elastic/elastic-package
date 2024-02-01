#!/usr/bin/env bash 

set -euxo pipefail

cleanup() {
    r=$?

    # Dump stack logs
    elastic-package stack dump -v --output build/elastic-stack-dump/system-test-flags

    # Take down the stack
    elastic-package stack down -v

    # Clean used resources
    for d in test/packages/*/*/; do
        (
        cd "$d"
        elastic-package clean -v
        )
    done

    exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Run default stack version
elastic-package stack up -v -d


pushd test/packages/parallel/nginx/ > /dev/null

elastic-package test system -v \
--config-file "$(pwd)/data_stream/access/_dev/test/system/test-default-config.yml" \
--setup

FOLDER_NAME="service_setup"
FOLDER_PATH="${HOME}/.elastic-package/stack/${FOLDER_NAME}"

if [ ! -d "$FOLDER_PATH" ]; then
    echo "Folder ${FOLDER_PATH} has not been created in --setup"
    exit 1
fi

if [ ! -f "${FOLDER_PATH}/orig-policy.json" ]; then
    echo "Missing orig-policy.json in ${FOLDER_NAME} folder"
    exit 1
fi

if [ ! -f "${FOLDER_PATH}/policy-setup.json" ]; then
    echo "Missing policy-setup.json in ${FOLDER_NAME} folder"
    exit 1
fi

if ! docker ps --format "{{.Image}}" | grep -q "elastic-package-service-nginx" ; then
    echo "Not find service docker container running after --setup process"
    exit 1
fi

# TODO  tests for --no-provision

elastic-package test system -v \
--config-file "$(pwd)/data_stream/access/_dev/test/system/test-default-config.yml" \
--tear-down

if [ -d "$FOLDER_PATH" ]; then
    echo "Folder ${FOLDER_NAME} has not been deleted in --tear-down"
    exit 1
fi

if docker ps --format "{{.Image}}" | grep -q "elastic-package-service-nginx" ; then
    echo "Service docker container is still running after --tear-down process"
    exit 1
fi

popd > /dev/null
