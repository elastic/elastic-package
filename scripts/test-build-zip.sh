#!/bin/bash
set -euxo pipefail

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

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

# Same as test-build-install-zip.sh: this integration needs the local stack registry
# and phase-2 build; building it here would hit production EPR for requires.input.
COMPOSABLE_INTEGRATION_DIR="test/packages/composable/02_ci_composable_integration/"

OLDPWD=$PWD
# Build packages
export ELASTIC_PACKAGE_SIGNER_PRIVATE_KEYFILE="$OLDPWD/scripts/gpg-private.asc"
ELASTIC_PACKAGE_SIGNER_PASSPHRASE=$(cat "$OLDPWD/scripts/gpg-pass.txt")
export ELASTIC_PACKAGE_SIGNER_PASSPHRASE
ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

go run ./scripts/gpgkey

for d in test/packages/*/*/; do
  # Added set +x in a sub-shell to avoid printing the testype command in the output
  # This helps to keep the CI output cleaner
  packageTestType=$(set +x ; testype "$d")
  # Packages in false_positives can have issues.
  if [ "${packageTestType}" == "false_positives" ]; then
    continue
  fi
  if [[ "${d}" == "${COMPOSABLE_INTEGRATION_DIR}" ]]; then
    echo "--- Skipping composable integration (built in test-build-install-zip phase 2 only): ${d}"
    continue
  fi
  echo "--- Building zip package: ${d}"
  elastic-package build -C "$d" --zip --sign -v
done

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/
