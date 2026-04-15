#!/bin/bash

set -euxo pipefail

ELASTIC_PACKAGE_CONFIG_FILE="${HOME}/.elastic-package/config.yml"
PREV_REGISTRY_URL=""
PACKAGE_REGISTRY_CI_OVERRIDE=0
COMPOSABLE_INTEGRATION_DIR="test/packages/composable/02_ci_composable_integration/"
COMPOSABLE_INPUT_DIR="test/packages/composable/01_ci_input_pkg/"
COMPOSABLE_ONLY=0

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

  local output_path="build/elastic-stack-dump/install-zip"
  if [ ${COMPOSABLE_ONLY} -eq 1 ]; then
      output_path="${output_path}-composable"
  fi
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
    echo "${0} [-c] [-s] [-v <stack_version>] [-h]"
    echo "Run test-install-zip suite"
    echo -e "\t-c: Run composable-only flow (build input dependency + composable integration; install composable zip only)."
    echo -e "\t-s: Use elastic-package stack shellinit to export environment variablles. By default, they should be exported manually."
    echo -e "\t-v <stack_version>: Speciy which Elastic Stack version to use. If not specified it will use the default version in elastic-package."
    echo -e "\t-h: Show this message"
}

USE_SHELLINIT=0
STACK_VERSION="default"
while getopts ":csv:h" o; do
    case "${o}" in
        c)
            COMPOSABLE_ONLY=1
            ;;
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

if [ ${COMPOSABLE_ONLY} -eq 1 ]; then
  echo "--- Building zip package (composable dependency): ${COMPOSABLE_INPUT_DIR}"
  elastic-package build -C "${COMPOSABLE_INPUT_DIR}"
else
  # Build packages (see test-build-install-zip.sh for composable phase-2 notes).
  for d in test/packages/*/*/; do
    # Added set +x in a sub-shell to avoid printing the testype command in the output
    # This helps to keep the CI output cleaner
    packageTestType=$(set +x ; testype "$d")
    # Packages in false_positives can have issues.
    if [ "${packageTestType}" == "false_positives" ]; then
      continue
    fi
    if [[ "${d}" == "${COMPOSABLE_INTEGRATION_DIR}" ]]; then
      echo "--- Skipping composable integration (phase-2 build): ${d}"
      continue
    fi
    echo "--- Building zip package: ${d}"
    elastic-package build -C "$d"
  done
fi

if [[ -f "${ELASTIC_PACKAGE_CONFIG_FILE}" ]]; then
  PREV_REGISTRY_URL=$(yq '.package_registry.base_url // ""' "${ELASTIC_PACKAGE_CONFIG_FILE}")
  yq eval --inplace '.package_registry.base_url = "https://127.0.0.1:8080"' "${ELASTIC_PACKAGE_CONFIG_FILE}"
else
  mkdir -p "$(dirname "${ELASTIC_PACKAGE_CONFIG_FILE}")"
  yq -n '.package_registry.base_url = "https://127.0.0.1:8080"' > "${ELASTIC_PACKAGE_CONFIG_FILE}"
fi
PACKAGE_REGISTRY_CI_OVERRIDE=1

echo "--- Phase-2 build: composable integration"
elastic-package build -C "${COMPOSABLE_INTEGRATION_DIR}"

# Remove unzipped built packages, leave .zip files
rm -r build/packages/*/

# Apply stack env only for the mode under test (shellinit vs manual exports).
if [ ${USE_SHELLINIT} -eq 1 ]; then
  eval "$(elastic-package stack shellinit)"
else
  export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
  export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
  export ELASTIC_PACKAGE_KIBANA_HOST=https://127.0.0.1:5601
  export ELASTIC_PACKAGE_CA_CERT=${HOME}/.elastic-package/profiles/default/certs/ca-cert.pem
fi

for zipFile in build/packages/*.zip; do
  if [ ${COMPOSABLE_ONLY} -eq 1 ]; then
    if [[ "$(basename "${zipFile}")" != ci_composable_integration-*.zip ]]; then
      continue
    fi
  fi
  installAndVerifyPackage "${zipFile}"
done
