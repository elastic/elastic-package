#!/bin/bash

set -euxo pipefail

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  # Dump stack logs
  elastic-package stack dump -v --output build/elastic-stack-dump/build-zip || true

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/*/*/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

trap cleanup EXIT

testype() {
  basename "$(dirname "$1")"
}

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
  echo "--- Building package: ${d}"
  elastic-package build -C "$d" --zip --sign -v
done

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/

echo "--- Prepare Elastic stack"
# Boot up the stack
elastic-package stack up -d -v

eval "$(elastic-package stack shellinit)"

# Install packages from working copy
for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  package_name=$(yq -r '.name' "${d}/manifest.yml")
  package_version=$(yq -r '.version' "${d}/manifest.yml")

  echo "--- Installing package: ${PACKAGE_NAME_VERSION}"
  elastic-package install -C "$d" -v

  # check that the package is installed
  curl -s \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f "${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${package_name}/${package_version}" | grep -q '"status":"installed"'
done
