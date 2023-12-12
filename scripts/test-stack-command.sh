#!/bin/bash

set -euxo pipefail

VERSION=${1:-default}
APM_SERVER_ENABLED=${APM_SERVER_ENABLED:-false}

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/stack/${VERSION}

  # Take down the stack
  elastic-package stack down -v

  if [ "${APM_SERVER_ENABLED}" = true ]; then
    # Create an apm-server profile and use it
    elastic-package profiles delete with-apm-server
  fi

  exit $r
}

default_version() {
  grep "DefaultStackVersion =" internal/install/stack_version.go | awk '{print $3}' | tr -d '"'
}

clean_status_output() {
  local output_file="$1"
  cat ${output_file} | grep "│" | tr -d ' '
}

trap cleanup EXIT

ARG_VERSION=""
EXPECTED_VERSION=$(default_version)
if [ "${VERSION}" != "default" ]; then
  ARG_VERSION="--version ${VERSION}"
  EXPECTED_VERSION=${VERSION}
fi

if [ "${APM_SERVER_ENABLED}" = true ]; then
  # Create an apm-server profile and use it
  profile=with-apm-server
  elastic-package profiles create -v ${profile}
  elastic-package profiles use ${profile}

  # Create the config and enable apm-server
  cat ~/.elastic-package/profiles/${profile}/config.yml.example - <<EOF > ~/.elastic-package/profiles/${profile}/config.yml
stack.apm_server_enabled: true
EOF
fi

OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}"
if [ "${APM_SERVER_ENABLED}" = true ]; then
  OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}_with_apm_server"
fi
mkdir -p ${OUTPUT_PATH_STATUS}

# Initial status empty
elastic-package stack status 2> ${OUTPUT_PATH_STATUS}/initial.txt
grep "\- No service running" ${OUTPUT_PATH_STATUS}/initial.txt

# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl --cacert ${ELASTIC_PACKAGE_CA_CERT} -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck

# Check status with running services
cat <<EOF > ${OUTPUT_PATH_STATUS}/expected_running.txt
Status of Elastic stack services:
╭──────────────────┬─────────┬───────────────────╮
│ SERVICE          │ VERSION │ STATUS            │
├──────────────────┼─────────┼───────────────────┤
│ elastic-agent    │ ${EXPECTED_VERSION}   │ running (healthy) │
│ elasticsearch    │ ${EXPECTED_VERSION}   │ running (healthy) │
│ fleet-server     │ ${EXPECTED_VERSION}   │ running (healthy) │
│ kibana           │ ${EXPECTED_VERSION}   │ running (healthy) │
│ package-registry │ latest  │ running (healthy) │
╰──────────────────┴─────────┴───────────────────╯
EOF

elastic-package stack status -v 2> ${OUTPUT_PATH_STATUS}/running.txt

# Remove spaces to avoid issues with spaces between columns
clean_status_output "${OUTPUT_PATH_STATUS}/expected_running.txt" > ${OUTPUT_PATH_STATUS}/expected_no_spaces.txt
clean_status_output "${OUTPUT_PATH_STATUS}/running.txt" > ${OUTPUT_PATH_STATUS}/running_no_spaces.txt

if [ "${APM_SERVER_ENABLED}" = true ]; then
  curl http://localhost:8200/
fi

diff -q ${OUTPUT_PATH_STATUS}/running_no_spaces.txt ${OUTPUT_PATH_STATUS}/expected_no_spaces.txt
