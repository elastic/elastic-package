#!/bin/bash

set -euxo pipefail

ELASTIC_PACKAGE_CONFIG_FILE="${HOME}/.elastic-package/config.yml"
PREV_REGISTRY_URL=""
PACKAGE_REGISTRY_CI_OVERRIDE=0
COMPOSABLE_INTEGRATION_DIR="test/packages/composable/02_ci_composable_integration/"

restore_package_registry_config() {
  if [[ "${PACKAGE_REGISTRY_CI_OVERRIDE}" -ne 1 ]]; then
    return 0
  fi
  if [[ ! -f "${ELASTIC_PACKAGE_CONFIG_FILE}" ]]; then
    return 0
  fi
  if [[ -n "${PREV_REGISTRY_URL}" ]]; then
    yq eval --inplace ".package_registry.base_url = \"${PREV_REGISTRY_URL}\"" "${ELASTIC_PACKAGE_CONFIG_FILE}" || true
  else
    yq eval --inplace 'del(.package_registry.base_url)' "${ELASTIC_PACKAGE_CONFIG_FILE}" || true
  fi
}

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  restore_package_registry_config

  # Dump stack logs
  # Required containers could not be running, so ignore the error
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

# Composable integration: requires ci_input_pkg from the registry. It is built in a
# second phase after the stack is up and package_registry.base_url points at the local EPR.
for d in test/packages/*/*/; do
  # Added set +x in a sub-shell to avoid printing the testype command in the output
  # This helps to keep the CI output cleaner
  packageTestType=$(set +x ; testype "$d")
  # Packages in false_positives can have issues.
  if [ "${packageTestType}" == "false_positives" ]; then
    continue
  fi
  if [[ "${d}" == "${COMPOSABLE_INTEGRATION_DIR}" ]]; then
    echo "--- Skipping composable integration (phase-2 build after stack is up): ${d}"
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

# Point elastic-package build at the stack's local package registry so phase-2 can
# download required input packages (see docs/howto/local_package_registry.md).
if [[ -f "${ELASTIC_PACKAGE_CONFIG_FILE}" ]]; then
  PREV_REGISTRY_URL=$(yq '.package_registry.base_url // ""' "${ELASTIC_PACKAGE_CONFIG_FILE}")
  yq eval --inplace '.package_registry.base_url = "https://127.0.0.1:8080"' "${ELASTIC_PACKAGE_CONFIG_FILE}"
else
  mkdir -p "$(dirname "${ELASTIC_PACKAGE_CONFIG_FILE}")"
  yq -n '.package_registry.base_url = "https://127.0.0.1:8080"' > "${ELASTIC_PACKAGE_CONFIG_FILE}"
fi
PACKAGE_REGISTRY_CI_OVERRIDE=1

echo "--- Phase-2 build: composable integration (requires local registry)"
elastic-package build -C "${COMPOSABLE_INTEGRATION_DIR}" --zip --sign -v

# Install packages from working copy
for d in test/packages/*/*/; do
  # Added set +x in a sub-shell to avoid printing the testype command in the output
  # This helps to keep the CI output cleaner
  packageTestType=$(set +x ; testype "$d")
  # Packages in false_positives can have issues.
  if [ "${packageTestType}" == "false_positives" ]; then
    continue
  fi
  package_name=$(yq -r '.name' "${d}/manifest.yml")
  package_version=$(yq -r '.version' "${d}/manifest.yml")

  echo "--- Installing package: ${package_name} (${package_version})"
  elastic-package install -C "$d" -v

  # check that the package is installed
  curl -s \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f "${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${package_name}/${package_version}" | grep -q '"status":"installed"'
done
