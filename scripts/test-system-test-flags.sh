#!/usr/bin/env bash

set -euxo pipefail

DEFAULT_AGENT_CONTAINER_NAME="elastic-package-service-docker-custom-agent"

cleanup() {
    local r
    local container_id
    r=$?

    # Dump stack logs
    elastic-package stack dump -v --output build/elastic-stack-dump/system-test-flags

    local container_id=""
    if is_service_container_running "${DEFAULT_AGENT_CONTAINER_NAME}" ; then
        container_id=$(container_ids "${DEFAULT_AGENT_CONTAINER_NAME}")
        docker rm -f "${container_id}"
    fi
    # remove if any service container
    if is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
        container_id=$(container_ids "${SERVICE_CONTAINER_NAME}")
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

trap cleanup EXIT

container_ids() {
    local container="$1"
    docker ps --format "{{ .ID}} {{ .Names}}" | grep "${container}" | awk '{print $1}'
}

is_service_container_running() {
    local container="$1"
    local container_ids=""

    container_ids=$(container_ids "${container}" | wc -l)

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

    if [ ! -f "${FOLDER_PATH}/service.json" ]; then
        echo "Missing service.json in folder ${FOLDER_PATH}"
        return 1
    fi

    return 0
}

run_tests_for_package() {
    local package_folder="$1"
    local service_name="$2"
    local config_file="$3"
    local variant="$4"
    local custom_agent="$5"
    local variant_flag=""
    if [[ $variant != "no variant" ]]; then
        variant_flag="--variant ${variant}"
    fi
    local package_name=""
    package_name="$(basename "${package_folder}")"

    # set global variable so it is accessible for cleanup function (trap)
    SERVICE_CONTAINER_NAME="elastic-package-service-${service_name}"
    AGENT_CONTAINER_NAME=""
    if [[ "${custom_agent}" == "true" ]]; then
        SERVICE_CONTAINER_NAME="${service_name}"
        AGENT_CONTAINER_NAME="${DEFAULT_AGENT_CONTAINER_NAME}"
    fi

    pushd "${package_folder}/" > /dev/null

    echo "--- [${package_name} - ${variant}] Setup service without tear-down"
    elastic-package test system -v \
        --report-format xUnit --report-output file \
        --config-file "${config_file}" \
        ${variant_flag} \
        --setup

    if ! temporal_files_exist ; then
        exit 1
    fi

    if ! is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
        echo "Not find service docker container running after --setup process"
        exit 1
    fi

    if [[ "${AGENT_CONTAINER_NAME}" != "" ]]; then
        if ! is_service_container_running "${AGENT_CONTAINER_NAME}"; then
            echo "Not find custom docker agent container running after --setup process"
            exit 1
        fi
    fi

    echo "--- [${package_name} - ${variant}] Run tests without provisioning"
    for i in $(seq 2); do
        echo "--- Iteration #${i} --no-provision"
        elastic-package test system -v \
            --report-format xUnit --report-output file \
            --no-provision

        # service docker needs to be running after this command
        if ! is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
            echo "Not find service docker container running after --no-provision process"
            exit 1
        fi
        if [[ "${AGENT_CONTAINER_NAME}" != "" ]]; then
            if ! is_service_container_running "${AGENT_CONTAINER_NAME}"; then
                echo "Not find custom docker agent container running after --no-provision process"
                exit 1
            fi
        fi

        if ! temporal_files_exist ; then
            exit 1
        fi
    done

    echo "--- [${package_name} - ${variant}] Run tear-down process"
    elastic-package test system -v \
        --report-format xUnit --report-output file \
        --tear-down

    if service_setup_folder_exists; then
        echo "State folder has not been deleted in --tear-down: ${FOLDER_PATH}"
        exit 1
    fi

    if is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
        echo "Service docker container is still running after --tear-down process"
        exit 1
    fi
    if [[ "${AGENT_CONTAINER_NAME}" != "" ]]; then
        if is_service_container_running "${AGENT_CONTAINER_NAME}"; then
            echo "Custom docker agent container still running after --tear-down process"
            exit 1
        fi
    fi

    popd > /dev/null
}

SERVICE_NETWORK_NAME="elastic-package-service_default"
# to be set the specific value in run_tests_for_package , required to be global
# so cleanup function could delete the container if is still running
SERVICE_CONTAINER_NAME="elastic-package-service-<package>"

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Run default stack version
echo "--- Start Elastic stack"
elastic-package stack up -v -d

FOLDER_PATH="${HOME}/.elastic-package/profiles/default/stack/state"

run_tests_for_package \
    "test/packages/parallel" \
    "nginx" \
    "data_stream/access/_dev/test/system/test-default-config.yml" \
    "no variant" \
    "false"

run_tests_for_package \
    "test/packages/parallel" \
    "sql_input" \
    "_dev/test/system/test-default-config.yml" \
    "mysql_8_0_13" \
    "false"

# this package has no service, so we introduced as a service name the one from the custom agent docker-custom-agent"
run_tests_for_package \
    "test/packages/with-custom-agent/auditd_manager" \
    "docker-custom-agent" \
    "./data_stream/auditd/_dev/test/system/test-default-config.yml" \
    "no variant" \
    "true"

run_tests_for_package \
    "test/packages/with-custom-agent/oracle" \
    "oracle" \
    "./data_stream/memory/_dev/test/system/test-memory-config.yml" \
    "no variant" \
    "true"
