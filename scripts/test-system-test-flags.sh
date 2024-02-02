#!/usr/bin/env bash

set -euxo pipefail

cleanup() {
    local r
    local container_id
    r=$?

    # Dump stack logs
    elastic-package stack dump -v --output build/elastic-stack-dump/system-test-flags

    # remove if any service container
    if is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
        container_id=$(docker ps --filter="ancestor=${SERVICE_CONTAINER_NAME}" -q)
        docker rm -f "${container_id}"
        docker network rm "${SERVICE_NETWORK_NAME}"
    fi

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

is_service_container_running() {
    local container="$1"
    local container_ids=""

    container_ids=$(docker ps --filter="ancestor=${container}" -q | wc -l)

    if [ "${container_ids}" -eq 1 ] ; then
        return 0
    fi
    return 1
}

service_setup_folder_exists() {
    if [ ! -d "$FOLDER_PATH" ]; then
        echo "Folder ${FOLDER_PATH} does not exist"
        return 1
    fi
    return 0
}


temporal_files_exist() {
    if ! service_setup_folder_exists ; then
        return 1
    fi

    if [ ! -f "${FOLDER_PATH}/orig-policy.json" ]; then
        echo "Missing orig-policy.json in ${FOLDER_NAME} folder"
        return 1
    fi

    if [ ! -f "${FOLDER_PATH}/policy-setup.json" ]; then
        echo "Missing policy-setup.json in ${FOLDER_NAME} folder"
        return 1
    fi
    return 0
}

trap cleanup EXIT

SERVICE_CONTAINER_NAME="elastic-package-service-nginx"
SERVICE_NETWORK_NAME="elastic-package-service_default"

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Run default stack version
echo "--- Start Elastic stack"
elastic-package stack up -v -d


pushd test/packages/parallel/nginx/ > /dev/null

echo "--- Setup service without tear-down"
elastic-package test system -v \
    --report-format xUnit --report-output file \
    --config-file "$(pwd)/data_stream/access/_dev/test/system/test-default-config.yml" \
    --setup

FOLDER_NAME="service_setup"
FOLDER_PATH="${HOME}/.elastic-package/stack/${FOLDER_NAME}"

if ! temporal_files_exist ; then
    exit 1
fi

if ! is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
    echo "Not find service docker container running after --setup process"
    exit 1
fi

echo "--- Run tests without provisioning"
for i in $(seq 3); do
    echo "--- Iteration #${i} --no-provision"
    elastic-package test system -v \
        --report-format xUnit --report-output file \
        --config-file "$(pwd)/data_stream/access/_dev/test/system/test-default-config.yml" \
        --no-provision

    # service docker needs to be running after this command
    if ! is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
        echo "Not find service docker container running after --no-provision process"
        exit 1
    fi

    if ! temporal_files_exist ; then
        exit 1
    fi
done

echo "--- Run tear-down process"
elastic-package test system -v \
    --report-format xUnit --report-output file \
    --config-file "$(pwd)/data_stream/access/_dev/test/system/test-default-config.yml" \
    --tear-down

if service_setup_folder_exists; then
    echo "Folder ${FOLDER_NAME} has not been deleted in --tear-down"
    exit 1
fi

if is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
    echo "Service docker container is still running after --tear-down process"
    exit 1
fi

popd > /dev/null
