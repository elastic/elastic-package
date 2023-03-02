#!/bin/bash

set -euxo pipefail

STACK_VERSION=${1:-default}

cleanup() {
  local r=$?

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

installAndVerifyPackage() {
  local zipFile="$1"
  local PACKAGE_NAME_VERSION=$(basename ${zipFile} .zip)

  elastic-package install -v --zip ${zipFile}

  # check that the package is installed
  curl -s \
    -u ${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD} \
    --cacert ${ELASTIC_PACKAGE_CA_CERT} \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f ${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${PACKAGE_NAME_VERSION} | grep -q '"status":"installed"'
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
  installAndVerifyPackage ${zipFile}
done

# try to install one package without elastic-package stack shellinit
unset ELASTIC_PACKAGE_KIBANA_HOST
unset ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME
unset ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD
unset ELASTIC_PACKAGE_CA_CERT

export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
export ELASTIC_PACKAGE_KIBANA_HOST=https://127.0.0.1:5601
export ELASTIC_PACKAGE_CA_CERT=${HOME}/.elastic-package/profiles/default/certs/ca-cert.pem

zipFile="build/packages/$(ls -rt build/packages/ | tail -n 1)"

installAndVerifyPackage ${zipFile}
