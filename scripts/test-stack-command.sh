#!/bin/bash

set -euxo pipefail

elastic-package stack up -d

eval "$(elastic-package stack shellinit)"
curl -f ${ELASTIC_PACKAGE_KIBANA_HOST}/login | grep kbn-injected-metadata >/dev/null # healthcheck

elastic-package stack down
