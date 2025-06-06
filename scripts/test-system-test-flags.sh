#!/usr/bin/env bash

set -euxo pipefail

cleanup() {
    local r=$?
    local container_id=""
    local agent_ids=""
    if [ "${r}" -ne 0 ]; then
      # Ensure that the group where the failure happened is opened.
      echo "^^^ +++"
    fi

    echo "~~~ elastic-package cleanup"

    # Dump stack logs
    if [ "${ELASTIC_PACKAGE_STARTED}" -eq 1 ]; then
        elastic-package stack dump -v --output build/elastic-stack-dump/system-test-flags
    fi

    if is_service_container_running "${DEFAULT_AGENT_CONTAINER_NAME}" ; then
        container_id=$(container_ids "${DEFAULT_AGENT_CONTAINER_NAME}")
        docker rm -f "${container_id}"
    fi

    # remove independent Elastic Agents
    agent_ids=$(container_ids "elastic-package-agent" || echo "")
    if [[ "${agent_ids}" != "" ]]; then
        for agent_id in ${agent_ids}; do
            docker rm -f "${agent_id}"
        done
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
        elastic-package clean -C "$d" -v
    done

    exit $r
}

trap cleanup EXIT

container_ids() {
    local container="$1"
    docker ps --format "{{ .ID}} {{ .Names}}" | grep -E "${container}" | awk '{print $1}'
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

service_state_folder_exists() {
    if [ ! -d "$FOLDER_PATH" ]; then
        echo "Folder ${FOLDER_PATH} does not exist"
        return 1
    fi
    return 0
}


temporal_files_exist() {
    if ! service_state_folder_exists ; then
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

    # state folder is kept after tear-down but there should not exist the service.json
    if ! service_state_folder_exists; then
        echo "State folder has been deleted in --tear-down: ${FOLDER_PATH}"
        return 1
    fi

    if temporal_files_exist ; then
        echo "Service state has not been deleted in --tear-down: ${FOLDER_PATH}"
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

    # TODO: oracle package still uses custom agent deployer
    if [[ "${package_name}" != "oracle" && "${ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT:-"false"}" == "true" ]]; then
        # set prefix of the independent Elastic Agents
        AGENT_CONTAINER_NAME="elastic-package-agent-${package_name}"
    fi

    pushd "${package_folder}" > /dev/null

    echo "--- [${package_name} - ${variant}] Setup service without tear-down"
    if ! elastic-package test system -v \
        --report-format xUnit --report-output file \
        --config-file "${config_file}" \
        ${variant_flag} \
        --setup ; then
        return 1
    fi

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
        if ! elastic-package test system -v \
            --report-format xUnit --report-output file \
            --no-provision ; then
            return 1
        fi

        # Tests after --no-provision
        if ! tests_for_no_provision \
            "${service_deployer_type}" \
            "${SERVICE_CONTAINER_NAME}" \
            "${AGENT_CONTAINER_NAME}" ; then
            return 1
        fi
    done

    echo "--- [${package_name} - ${variant}] Run tear-down process"
    if ! elastic-package test system -v \
        --report-format xUnit --report-output file \
        --tear-down ; then
        return 1
    fi

    if ! tests_for_tear_down \
        "${service_deployer_type}" \
        "${SERVICE_CONTAINER_NAME}" \
        "${AGENT_CONTAINER_NAME}" ; then
        return 1
    fi

    popd > /dev/null
}


# Set variables depending on whether or not independent Elastic Agents are running
DEFAULT_AGENT_CONTAINER_NAME="elastic-package-service-[0-9]{5}-docker-custom-agent"
service_deployer_type="docker"
service_prefix='elastic-package-service-[0-9]{5}'
if [[ "${ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT:-"false"}" == "true" ]]; then
    service_deployer_type="agent"
fi

# to be set the specific value in run_tests_for_package , required to be global
# so cleanup function could delete the container if is still running
SERVICE_CONTAINER_NAME=""

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH
ELASTIC_PACKAGE_STARTED=0

# Run default stack version
echo "--- Start Elastic stack"
elastic-package stack up -v -d
ELASTIC_PACKAGE_STARTED=1

elastic-package stack status

FOLDER_PATH="${HOME}/.elastic-package/profiles/default/stack/state"

# docker service deployer
if ! run_tests_for_package \
    "test/packages/parallel/nginx" \
    "${service_prefix}-nginx" \
    "data_stream/access/_dev/test/system/test-default-config.yml" \
    "no variant" \
    "${service_deployer_type}" ; then

    exit 1
fi

if ! run_tests_for_package \
    "test/packages/parallel/sql_input" \
    "${service_prefix}-sql_input" \
    "_dev/test/system/test-default-config.yml" \
    "mysql_8_0_13" \
    "${service_deployer_type}" ; then

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
