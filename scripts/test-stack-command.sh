#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

set -euxo pipefail

source "${SCRIPT_DIR}/stack_helpers.sh"

VERSION=${1:-default}
APM_SERVER_ENABLED=${APM_SERVER_ENABLED:-false}
SELF_MONITOR_ENABLED=${SELF_MONITOR_ENABLED:-false}
ELASTIC_SUBSCRIPTION=${ELASTIC_SUBSCRIPTION:-""}
AGENT_VERSION_FLAG=${AGENT_VERSION_FLAG:-""}

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if is_stack_created; then
    # Dump stack logs
    # Required containers could not be running, so ignore the error
    elastic-package stack dump -v --output "build/elastic-stack-dump/stack/${VERSION}" || true

    # Take down the stack
    elastic-package stack down -v
  fi

  if [ "${APM_SERVER_ENABLED}" = true ]; then
    elastic-package profiles delete with-apm-server
  fi

  if [ "${SELF_MONITOR_ENABLED}" = true ]; then
    elastic-package profiles delete with-self-monitor
  fi

  if [[ "${ELASTIC_SUBSCRIPTION}" != "" ]]; then
    elastic-package profiles delete with-elastic-subscription
  fi

  exit $r
}

default_version() {
  grep "DefaultStackVersion =" internal/install/stack_version.go | awk '{print $3}' | tr -d '"'
}

clean_status_output() {
  local output_file="$1"
  cat "${output_file}" | grep "│" | tr -d ' '
}

trap cleanup EXIT

ARG_VERSION=""
EXPECTED_VERSION=$(default_version)
if [ "${VERSION}" != "default" ]; then
  ARG_VERSION="--version ${VERSION}"
  EXPECTED_VERSION=${VERSION}
fi
if [ "${AGENT_VERSION_FLAG}" != "" ]; then
    ARG_VERSION="${ARG_VERSION} --agent-version ${AGENT_VERSION_FLAG}"
    echo "Using agent version override flag: ${AGENT_VERSION_FLAG}"
fi

OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}"

if [ "${APM_SERVER_ENABLED}" = true ]; then
  echo "--- Create APM server profile"
  OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}_with_apm_server"

  # Create an apm-server profile and use it
  profile=with-apm-server
  elastic-package profiles create -v ${profile}
  elastic-package profiles use ${profile}

  # Create the config and enable apm-server
  cat ~/.elastic-package/profiles/${profile}/config.yml.example - <<EOF > ~/.elastic-package/profiles/${profile}/config.yml
stack.apm_enabled: true
EOF
fi

if [ "${SELF_MONITOR_ENABLED}" = true ]; then
  echo "--- Create self-monitor profile"
  # Create a self-monitor profile and use it
  profile=with-self-monitor
  elastic-package profiles create -v ${profile}
  elastic-package profiles use ${profile}

  cat ~/.elastic-package/profiles/${profile}/config.yml.example - <<EOF > ~/.elastic-package/profiles/${profile}/config.yml
stack.self_monitor_enabled: true
EOF
fi

if [[ "${ELASTIC_SUBSCRIPTION}" != "" ]]; then
  echo "--- Create elastic subscription profile"
  profile=with-elastic-subscription
  elastic-package profiles create -v ${profile}
  elastic-package profiles use ${profile}

  cat ~/.elastic-package/profiles/${profile}/config.yml.example - <<EOF > ~/.elastic-package/profiles/${profile}/config.yml
stack.elastic_subscription: ${ELASTIC_SUBSCRIPTION}
EOF
fi

mkdir -p "${OUTPUT_PATH_STATUS}"

echo "--- Check initial Elastic stack status"
# Initial status empty
elastic-package stack status 2> "${OUTPUT_PATH_STATUS}/initial.txt"
grep "\- No service running" "${OUTPUT_PATH_STATUS}/initial.txt"

EXPECTED_AGENT_VERSION="${EXPECTED_VERSION}"
# Override agent version if flag is set
if [ "${AGENT_VERSION_FLAG}" != "" ]; then
    EXPECTED_AGENT_VERSION="${AGENT_VERSION_FLAG}"
fi
if [[ "${EXPECTED_AGENT_VERSION}" =~ ^7\.17 ]] ; then
    # Required starting with STACK_VERSION 7.17.21
    export ELASTIC_AGENT_IMAGE_REF_OVERRIDE="docker.elastic.co/beats/elastic-agent-complete:${EXPECTED_AGENT_VERSION}-amd64"
    EXPECTED_AGENT_VERSION="${EXPECTED_AGENT_VERSION}-amd64"
    echo "Override elastic-agent docker image: ${ELASTIC_AGENT_IMAGE_REF_OVERRIDE}"
fi

echo "--- Prepare Elastic stack"
# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

echo "--- Check Elastic stack status"
# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl --cacert "${ELASTIC_PACKAGE_CA_CERT}" -f "${ELASTIC_PACKAGE_KIBANA_HOST}/login" | grep kbn-injected-metadata >/dev/null # healthcheck

# Check status with running services
cat <<EOF > "${OUTPUT_PATH_STATUS}/expected_running.txt"
Status of Elastic stack services:
╭──────────────────┬─────────┬───────────────────╮
│ SERVICE          │ VERSION │ STATUS            │
├──────────────────┼─────────┼───────────────────┤
│ elastic-agent    │ ${EXPECTED_AGENT_VERSION}   │ running (healthy) │
│ elasticsearch    │ ${EXPECTED_VERSION}   │ running (healthy) │
│ fleet-server     │ ${EXPECTED_AGENT_VERSION}   │ running (healthy) │
│ kibana           │ ${EXPECTED_VERSION}   │ running (healthy) │
│ package-registry │ latest  │ running (healthy) │
╰──────────────────┴─────────┴───────────────────╯
EOF

NO_COLOR=true elastic-package stack status -v 2> "${OUTPUT_PATH_STATUS}/running.txt"

# Remove spaces to avoid issues with spaces between columns
clean_status_output "${OUTPUT_PATH_STATUS}/expected_running.txt" > "${OUTPUT_PATH_STATUS}/expected_no_spaces.txt"
clean_status_output "${OUTPUT_PATH_STATUS}/running.txt" > "${OUTPUT_PATH_STATUS}/running_no_spaces.txt"

diff -q "${OUTPUT_PATH_STATUS}/running_no_spaces.txt" "${OUTPUT_PATH_STATUS}/expected_no_spaces.txt"

if [ "${APM_SERVER_ENABLED}" = true ]; then
  echo "--- Check APM server status"
  curl http://localhost:8200/
fi

if [ "${SELF_MONITOR_ENABLED}" = true ]; then
  echo "--- Check self-monitor status"
  # Check that there is some data in the system indexes.
  curl -s -S --retry 5 --retry-all-errors --fail \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -f "${ELASTIC_PACKAGE_ELASTICSEARCH_HOST}/metrics-system.*/_search?allow_no_indices=false&size=0"
fi

echo "Check Elastic stack license"
subscription=$(curl -s -S \
  -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
  --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
  -f "${ELASTIC_PACKAGE_ELASTICSEARCH_HOST}/_license" |jq -r '.license.type')

expected_subscription="trial"
if [[ "${ELASTIC_SUBSCRIPTION}" != "" ]]; then
    expected_subscription="${ELASTIC_SUBSCRIPTION}"
fi

if [[ "${subscription}" != "${expected_subscription}" ]]; then
    echo "Unexpected \"${subscription}\" subscription found, but expected \"${expected_subscription}\""
    exit 1
fi
