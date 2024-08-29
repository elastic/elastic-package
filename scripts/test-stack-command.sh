#!/bin/bash

set -euxo pipefail

VERSION=${1:-default}
APM_SERVER_ENABLED=${APM_SERVER_ENABLED:-false}
SELF_MONITOR_ENABLED=${SELF_MONITOR_ENABLED:-false}

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output "build/elastic-stack-dump/stack/${VERSION}"

  # Take down the stack
  elastic-package stack down -v

  if [ "${APM_SERVER_ENABLED}" = true ]; then
    elastic-package profiles delete with-apm-server
  fi

  if [ "${SELF_MONITOR_ENABLED}" = true ]; then
    elastic-package profiles delete with-self-monitor
  fi

  exit $r
}

default_version() {
  grep "DefaultStackVersion =" internal/install/stack_version.go | awk '{print $3}' | tr -d '"'
}

clean_status_output() {
  local output_file="$1"
  # This removes the 'IMAGE BUILD DATE" and 'VCS REF' columns and
  # removes the whitespace between columns.
  grep "│" "${output_file}" \
    | sed 's/│/|/g' \
    | cut -d '|' -f 1-4,7- \
    | tr -d ' '
}

trap cleanup EXIT

ARG_VERSION=""
EXPECTED_VERSION=$(default_version)
if [ "${VERSION}" != "default" ]; then
  ARG_VERSION="--version ${VERSION}"
  EXPECTED_VERSION=${VERSION}
fi

OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}"

if [ "${APM_SERVER_ENABLED}" = true ]; then
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
  # Create a self-monitor profile and use it
  profile=with-self-monitor
  elastic-package profiles create -v ${profile}
  elastic-package profiles use ${profile}

  cat ~/.elastic-package/profiles/${profile}/config.yml.example - <<EOF > ~/.elastic-package/profiles/${profile}/config.yml
stack.self_monitor_enabled: true
EOF
fi

mkdir -p "${OUTPUT_PATH_STATUS}"

# Initial status empty
elastic-package stack status 2> "${OUTPUT_PATH_STATUS}/initial.txt"
grep "\- No service running" "${OUTPUT_PATH_STATUS}/initial.txt"

# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl --cacert "${ELASTIC_PACKAGE_CA_CERT}" -f "${ELASTIC_PACKAGE_KIBANA_HOST}/login" | grep kbn-injected-metadata >/dev/null # healthcheck

# Check status with running services
cat <<EOF > "${OUTPUT_PATH_STATUS}/expected_running.txt"
Status of Elastic stack services:
╭──────────────────┬─────────────────────┬───────────────────┬───────────────────┬────────────╮
│ SERVICE          │ VERSION             │ STATUS            │ IMAGE BUILD DATE  │ VCS REF    │
├──────────────────┼─────────────────────┼───────────────────┼───────────────────┼────────────┤
│ elastic-agent    │ ${EXPECTED_VERSION} │ running (healthy) │ 2024-08-22T02:44Z │ b96a4ca8fa │
│ elasticsearch    │ ${EXPECTED_VERSION} │ running (healthy) │ 2024-08-22T13:26Z │ 1362d56865 │
│ fleet-server     │ ${EXPECTED_VERSION} │ running (healthy) │ 2024-08-22T02:44Z │ b96a4ca8fa │
│ kibana           │ ${EXPECTED_VERSION} │ running (healthy) │ 2024-08-22T11:09Z │ cdcdfddd3f │
│ package-registry │ latest              │ running (healthy) │                   │            │
╰──────────────────┴─────────────────────┴───────────────────┴───────────────────┴────────────╯
EOF

elastic-package stack status -v 2> "${OUTPUT_PATH_STATUS}/running.txt"

# Remove dates, commit IDs, and spaces to avoid issues.
clean_status_output "${OUTPUT_PATH_STATUS}/expected_running.txt" > "${OUTPUT_PATH_STATUS}/expected_no_spaces.txt"
clean_status_output "${OUTPUT_PATH_STATUS}/running.txt" > "${OUTPUT_PATH_STATUS}/running_no_spaces.txt"

if [ "${APM_SERVER_ENABLED}" = true ]; then
  curl http://localhost:8200/
fi

if [ "${SELF_MONITOR_ENABLED}" = true ]; then
  # Check that there is some data in the system indexes.
  curl -s -S --retry 5 --retry-all-errors --fail \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -f "${ELASTIC_PACKAGE_ELASTICSEARCH_HOST}/metrics-system.*/_search?allow_no_indices=false&size=0"
fi

diff -q "${OUTPUT_PATH_STATUS}/running_no_spaces.txt" "${OUTPUT_PATH_STATUS}/expected_no_spaces.txt"
