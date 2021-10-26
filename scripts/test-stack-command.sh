#!/bin/bash

set -euxo pipefail

VERSION=${1}

if [ "${VERSION}" == "" ]; then
  echo "stack version required" && exit 1
fi

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/stack/${VERSION}

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/*/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

# Update the stack
elastic-package stack update --version ${VERSION} -v

# Boot up the stack
elastic-package stack up -d --version ${VERSION} -v

# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck
