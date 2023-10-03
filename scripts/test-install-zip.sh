#!/bin/bash

set -euxo pipefail

cleanup() {
  local r=$?

  local output_path="build/elastic-stack-dump/install-zip"
  if [ ${USE_SHELLINIT} -eq 1 ]; then
      output_path="${output_path}-shellinit"
  fi

  # Dump stack logs
  elastic-package stack dump -v --output ${output_path}

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

testype() {
  echo $(basename $(dirname $1))
}

trap cleanup EXIT

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

usage() {
    echo "${0} [-s] [-h]"
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

# Update the stack
elastic-package stack update -v ${ARG_VERSION}

# Boot up the stack
elastic-package stack up -d -v ${ARG_VERSION}

export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
OLDPWD=$PWD

# Build packages
for d in test/packages/*/*/; do
  # Packages in false_positives can have issues.
  if [ "$(testype $d)" == "false_positives" ]; then
    continue
  fi
  (
    cd $d
    elastic-package build
  )
done
cd $OLDPWD

# Remove unzipped built packages, leave .zip files
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
  installAndVerifyPackage ${zipFile}
done
