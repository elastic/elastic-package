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

trap cleanup EXIT

ARG_VERSION=""
if [ "${VERSION}" != "default" ]; then
  ARG_VERSION="--version ${VERSION}"
fi

# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck
