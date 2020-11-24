#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

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

# Boot up the stack
elastic-package stack up -d -v

# Verify it's accessible
eval "$(elastic-package stack shellinit)"
curl -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck
