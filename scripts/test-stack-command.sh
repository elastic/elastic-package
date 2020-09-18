#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?
  elastic-package stack down -v
  exit $r
}

trap cleanup EXIT

elastic-package stack up -d -v

eval "$(elastic-package stack shellinit)"
curl -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck
