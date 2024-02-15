#!/usr/bin/env bash

set -euxo pipefail

DEFAULT_AGENT_CONTAINER_NAME="elastic-package-service-docker-custom-agent"

cleanup() {
    local r=$?
    local container_id

    # Dump stack logs
    elastic-package stack dump -v --output build/elastic-stack-dump/system-test-flags

    local container_id=""
    if is_service_container_running "${DEFAULT_AGENT_CONTAINER_NAME}" ; then
        container_id=$(container_ids "${DEFAULT_AGENT_CONTAINER_NAME}")
        docker rm -f "${container_id}"
    fi
    # remove if any service container
    if [[ "${SERVICE_CONTAINER_NAME}" != "" ]]; then
        if is_service_container_running "${SERVICE_CONTAINER_NAME}"; then
            container_id=$(container_ids "${SERVICE_CONTAINER_NAME}")
            docker rm -f "${container_id}"
        fi
    fi

    kind delete cluster || true

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

is_network_created() {
    local network="$1"

    docker network ls --format "{{ .Name }}" | grep -q "${network}"
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

tests_for_setup() {
    local service_deployer_type=$1
    local service_container=$2
    local agent_container=$3

    if ! temporal_files_exist ; then
        return 1
    fi

    # TODO Add some other test for packages using kind
    if [[ "${service_deployer_type}" == "kind" ]]; then
        if ! is_network_created "kind" ; then
            echo "Missing docker network to connect to kind cluster"
            return 1
        fi
    fi

    if [[ "${service_deployer_type}" == "docker" || "${service_deployer_type}" == "agent" ]] ; then
        if ! is_service_container_running "${service_container}"; then
            echo "Not find service docker container running after --setup process"
            return 1
        fi
    fi

    if [[ "${service_deployer_type}" == "agent" ]] ; then
        if ! is_service_container_running "${agent_container}"; then
            echo "Not find custom docker agent container running after --setup process"
            return 1
        fi
    fi
    return 0
}

tests_for_no_provision() {
    local service_deployer_type=$1
    local service_container=$2
    local agent_container=$3

    # TODO Add some other test for packages using kind

    # service docker needs to be running after this command
    if [[ "${service_deployer_type}" == "docker" || "${service_deployer_type}" == "agent" ]] ; then
        if ! is_service_container_running "${service_container}"; then
            echo "Not find service docker container running after --no-provision process"
            return 1
        fi
    fi
    if [[ "${service_deployer_type}" == "agent" ]] ; then
        if ! is_service_container_running "${agent_container}"; then
            echo "Not find custom docker agent container running after --no-provision process"
            return 1
        fi
    fi

    if ! temporal_files_exist ; then
        return 1
    fi

    return 0
}

tests_for_tear_down() {
    local service_deployer_type=$1
    local service_container=$2
    local agent_container=$3

    # TODO Add some other test for packages using kind

    if service_setup_folder_exists; then
        echo "State folder has not been deleted in --tear-down: ${FOLDER_PATH}"
        return 1
    fi

    if [[ "${service_deployer_type}" == "docker" || "${service_deployer_type}" == "agent" ]] ; then
        if is_service_container_running "${service_container}"; then
            echo "Service docker container is still running after --tear-down process"
            return 1
        fi
    fi
    if [[ "${service_deployer_type}" == "agent" ]] ; then
        if is_service_container_running "${agent_container}"; then
            echo "Custom docker agent container still running after --tear-down process"
            return 1
        fi
    fi
    return 0
}

run_tests_for_package() {
    local package_folder="$1"
    local service_name="$2"
    local config_file="$3"
    local variant="$4"
    local service_deployer_type="$5"
    local variant_flag=""
    if [[ "$variant" != "no variant" ]]; then
        variant_flag="--variant ${variant}"
    fi
    local package_name=""
    package_name="$(basename "${package_folder}")"

    # set global variable so it is accessible for cleanup function (trap)
    SERVICE_CONTAINER_NAME="${service_name}"
    AGENT_CONTAINER_NAME=""
    if [[ "${service_deployer_type}" == "agent" ]]; then
        AGENT_CONTAINER_NAME="${DEFAULT_AGENT_CONTAINER_NAME}"
    fi

    pushd "${package_folder}" > /dev/null

    echo "--- [${package_name} - ${variant}] Setup service without tear-down"
    elastic-package test system -v \
        --report-format xUnit --report-output file \
        --config-file "${config_file}" \
        ${variant_flag} \
        --setup

    # Tests after --setup
    if ! tests_for_setup \
        "${service_deployer_type}" \
        "${SERVICE_CONTAINER_NAME}" \
        "${AGENT_CONTAINER_NAME}" ; then
        return 1
    fi

    echo "--- [${package_name} - ${variant}] Run tests without provisioning"
    for i in $(seq 2); do
        echo "--- Iteration #${i} --no-provision"
        elastic-package test system -v \
            --report-format xUnit --report-output file \
            --no-provision

        # Tests after --no-provision
        if ! tests_for_no_provision \
            "${service_deployer_type}" \
            "${SERVICE_CONTAINER_NAME}" \
            "${AGENT_CONTAINER_NAME}" ; then
            return 1
        fi
    done

    echo "--- [${package_name} - ${variant}] Run tear-down process"
    elastic-package test system -v \
        --report-format xUnit --report-output file \
        --tear-down

    if ! tests_for_tear_down \
        "${service_deployer_type}" \
        "${SERVICE_CONTAINER_NAME}" \
        "${AGENT_CONTAINER_NAME}" ; then
        return 1
    fi

    popd > /dev/null
}


# to be set the specific value in run_tests_for_package , required to be global
# so cleanup function could delete the container if is still running
SERVICE_CONTAINER_NAME=""

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Run default stack version
echo "--- Start Elastic stack"
elastic-package stack up -v -d

elastic-package stack status

FOLDER_PATH="${HOME}/.elastic-package/profiles/default/stack/state"

# docker service deployer
if ! run_tests_for_package \
    "test/packages/parallel/nginx" \
    "elastic-package-service-nginx" \
    "data_stream/access/_dev/test/system/test-default-config.yml" \
    "no variant" \
    "docker" ; then

    exit 1
fi

if ! run_tests_for_package \
    "test/packages/parallel/sql_input" \
    "elastic-package-service-sql_input" \
    "_dev/test/system/test-default-config.yml" \
    "mysql_8_0_13" \
    "docker" ; then

    exit 1
fi

# Custom agents service deployer
if ! run_tests_for_package \
    "test/packages/with-custom-agent/oracle" \
    "oracle" \
    "./data_stream/memory/_dev/test/system/test-memory-config.yml" \
    "no variant" \
    "agent" ; then

    exit 1
fi

# Kind service deployer
echo "--- Create kind cluster"
kind create cluster --config "$PWD/scripts/kind-config.yaml"

if ! run_tests_for_package \
    "test/packages/with-kind/kubernetes" \
    "" \
    "./data_stream/state_pod/_dev/test/system/test-default-config.yml" \
    "no variant" \
    "kind" ; then

    exit 1
fi
