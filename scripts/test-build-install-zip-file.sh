#!/bin/bash

set -euxo pipefail

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  local output_path="build/elastic-stack-dump/install-zip"
  if [ ${USE_SHELLINIT} -eq 1 ]; then
      output_path="${output_path}-shellinit"
  fi

  # Dump stack logs
  # Required containers could not be running, so ignore the error
  elastic-package stack dump -v --output ${output_path} || true

  # Take down the stack
  elastic-package stack down -v

  for d in test/packages/*/*/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

testype() {
  basename "$(dirname "$1")"
}

trap cleanup EXIT

installAndVerifyPackage() {
  local zipFile="$1"

  local PACKAGE_NAME_VERSION=""
  PACKAGE_NAME_VERSION=$(basename "${zipFile}" .zip)

  # Replace dash with a slash in the file name to match the API endpoint format
  # e.g. "apache-999.999.999" becomes "apache/999.999.999"
  PACKAGE_NAME_VERSION="${PACKAGE_NAME_VERSION//-/\/}"

  echo "--- Installing package: ${PACKAGE_NAME_VERSION}"
  elastic-package install -v --zip "${zipFile}"

  # check that the package is installed
  curl -s \
    -u "${ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME}:${ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD}" \
    --cacert "${ELASTIC_PACKAGE_CA_CERT}" \
    -H 'content-type: application/json' \
    -H 'kbn-xsrf: true' \
    -f "${ELASTIC_PACKAGE_KIBANA_HOST}/api/fleet/epm/packages/${PACKAGE_NAME_VERSION}" | grep -q '"status":"installed"'
}

usage() {
    echo "${0} [-s] [-v <stack_version>] [-h]"
    echo "Run test-install-zip suite"
    echo -e "\t-s: Use elastic-package stack shellinit to export environment variablles. By default, they should be exported manually."
    echo -e "\t-v <stack_version>: Speciy which Elastic Stack version to use. If not specified it will use the default version in elastic-package."
    echo -e "\t-h: Show this message"
}

USE_SHELLINIT=0
STACK_VERSION="default"
while getopts ":sv:h" o; do
    case "${o}" in
        s)
            USE_SHELLINIT=1
            ;;
        v)
            STACK_VERSION=${OPTARG}
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

ARG_VERSION=""
if [ "${STACK_VERSION}" != "default" ]; then
  ARG_VERSION="--version ${STACK_VERSION}"
fi

echo "--- Prepare Elastic stack"
# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Configure local registry so packages with requires.input can resolve dependencies
# The local registry is at https://localhost:8080 (served from build/packages/ zips)
# RegistryClientOptions automatically skips TLS verification for localhost
echo "--- Configure package registry to use local stack"
mkdir -p ~/.elastic-package
if [ -f ~/.elastic-package/config.yml ]; then
  yq -i '.package_registry.base_url = "https://localhost:8080"' ~/.elastic-package/config.yml
else
  printf 'package_registry:\n  base_url: "https://localhost:8080"\n' > ~/.elastic-package/config.yml
fi

# Pass 1: Build packages that do NOT have requires.input
# (packages with requires.input need dependent zips present in the local registry first)
for d in test/packages/*/*/; do
  # Added set +x in a sub-shell to avoid printing the testype command in the output
  # This helps to keep the CI output cleaner
  packageTestType=$(set +x ; testype "$d")
  # Packages in false_positives can have issues.
  if [ "${packageTestType}" == "false_positives" ]; then
    continue
  fi
  if grep -q "^requires:" "${d}/manifest.yml" && grep -A5 "^requires:" "${d}/manifest.yml" | grep -q "input:"; then
    continue  # defer to second pass (needs dependency zips in local registry)
  fi
  echo "--- Building zip package: ${d}"
  elastic-package build -C "$d"
done

# Remove unzipped built packages from pass 1, leave .zip files
rm -r build/packages/*/

# Pass 2: Build packages WITH requires.input (local registry now serves pass-1 zips)
for d in test/packages/*/*/; do
  packageTestType=$(set +x ; testype "$d")
  if [ "${packageTestType}" == "false_positives" ]; then
    continue
  fi
  if ! grep -q "^requires:" "${d}/manifest.yml" || ! grep -A5 "^requires:" "${d}/manifest.yml" | grep -q "input:"; then
    continue  # already built in pass 1
  fi
  echo "--- Building zip package: ${d}"
  elastic-package build -C "$d"
done

# Remove unzipped built packages from pass 2, leave .zip files
rm -r build/packages/*/

if [ ${USE_SHELLINIT} -eq 1 ]; then
  eval "$(elastic-package stack shellinit)"
else
  export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
  export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
  export ELASTIC_PACKAGE_KIBANA_HOST=https://127.0.0.1:5601
  export ELASTIC_PACKAGE_CA_CERT=${HOME}/.elastic-package/profiles/default/certs/ca-cert.pem
fi

for zipFile in build/packages/*.zip; do
  installAndVerifyPackage "${zipFile}"
done
