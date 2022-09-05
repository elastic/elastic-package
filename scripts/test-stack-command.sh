#!/bin/bash

set -euxo pipefail

VERSION=${1:-default}

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/stack/${VERSION}

  # Take down the stack
  elastic-package stack down -v

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

OUTPUT_PATH_STATUS="build/elastic-stack-status/${VERSION}"
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

diff -q ${OUTPUT_PATH_STATUS}/running_no_spaces.txt ${OUTPUT_PATH_STATUS}/expected_no_spaces.txt
