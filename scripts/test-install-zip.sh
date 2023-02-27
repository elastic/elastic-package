#!/bin/bash

set -euxo pipefail

STACK_VERSION=${1:-default}

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/install-zip

  # Take down the stack
  elastic-package stack down -v

  for d in test/packages/*/*/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

ARG_VERSION=""
if [ "${STACK_VERSION}" != "default" ]; then
  ARG_VERSION="--version ${STACK_VERSION}"
fi

# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

# Verify it's accessible
eval "$(elastic-package stack shellinit)"

export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
OLDPWD=$PWD

# Build packages
for d in test/packages/*/*/; do
  (
    cd $d
    elastic-package build
  )
done
cd $OLDPWD

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/

# Install packages and verify
for zipFile in build/packages/*.zip; do
  PACKAGE_NAME_VERSION=$(basename ${zipFile} .zip)

  # check that the package is installed
  elastic-package install -v --zip ${zipFile}

  curl -s \
    -u ${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD} \
    --cacert ${ELASTIC_PACKAGE_CA_CERT} \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f ${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${PACKAGE_NAME_VERSION} | grep -q '"status":"installed"'
done
