#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/build-zip

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/*/*/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

testype() {
  basename "$(dirname "$1")"
}

trap cleanup EXIT

OLDPWD=$PWD
# Build packages
export ELASTIC_PACKAGE_SIGNER_PRIVATE_KEYFILE="$OLDPWD/scripts/gpg-private.asc"
ELASTIC_PACKAGE_SIGNER_PASSPHRASE=$(cat "$OLDPWD/scripts/gpg-pass.txt")
export ELASTIC_PACKAGE_SIGNER_PASSPHRASE
ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

go run ./scripts/gpgkey

for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  elastic-package build -C "$d" --zip --sign -v
done

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/

# Boot up the stack
elastic-package stack up -d -v

eval "$(elastic-package stack shellinit)"

# Install packages from working copy
for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi

  elastic-package install -C "$d" -v

  # check that the package is installed
  curl -s \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f "${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${PACKAGE_NAME_VERSION}" | grep -q '"status":"installed"'
done
