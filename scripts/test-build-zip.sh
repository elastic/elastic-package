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
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

testype() {
  echo $(basename $(dirname $1))
}

trap cleanup EXIT

OLDPWD=$PWD
# Build packages
export ELASTIC_PACKAGE_SIGNER_PRIVATE_KEYFILE="$OLDPWD/scripts/gpg-private.asc"
export ELASTIC_PACKAGE_SIGNER_PASSPHRASE=$(cat "$OLDPWD/scripts/gpg-pass.txt")
export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"

for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  (
    cd $d
    elastic-package build --zip --sign -v
  )
done
cd -

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/

# Boot up the stack
elastic-package stack up -d -v

# Install zipped packages
for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  (
    cd $d
    elastic-package install -v
  )
cd -
done
