#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  if [ "${SKIP_INSTALL}" -eq 0 ]; then
      # Dump stack logs
      elastic-package stack dump -v --output build/elastic-stack-dump/build-zip

      # Take down the stack
      elastic-package stack down -v
  fi

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

usage() {
    echo "${0} [-s] [-h]"
    echo "Run test-build-zip suite"
    echo -e "\t-s: Skip installation of the package."
    echo -e "\t-h: Show this message"
}

SKIP_INSTALL=0
while getopts ":sh" o; do
    case "${o}" in
        s)
            SKIP_INSTALL=1
            ;;
        h)
            usage
            exit 0
            ;;
        \?)
            echo "Invalid option ${OPTARG}"
            usage
            exit 1
            ;;
        :)
            echo "Missing argument for -${OPTARG}"
            usage
            exit 1
            ;;
    esac
done
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

if [[ "${SKIP_INSTALL}" == 1 ]]; then
    exit 0
fi

# Boot up the stack
elastic-package stack up -d -v

# Install zipped packages
for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  elastic-package install -C "$d" -v
done
