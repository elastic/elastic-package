#!/bin/bash
set -euo pipefail

cleanup() {
    cd $FIRST_PWD
    rm -rf tmp.elastic-package.*
}

trap cleanup EXIT

FIRST_PWD=$(pwd)

echo "Checking gsutil command..."
if ! command -v gsutil &> /dev/null ; then
    echo "⚠️  gsutil is not installed"
    exit 1
else
    echo "✅ gsutil is installed"
fi

source .buildkite/scripts/install_deps.sh

repoName() {
    # Example of URL: git@github.com:acme-inc/my-project.git
    local repoUrl=$1

    orgAndRepo=$(echo $repoUrl | cut -d':' -f 2)
    echo "$(basename ${orgAndRepo} .git)"
}

isAlreadyPublished() {
    local packageZip=$1

    if curl --head https://package-storage.elastic.co/artifacts/packages/${packageZip} | grep -q "HTTP/2 200" ; then
        return 0
    fi
    return 1
}

REPO_NAME=$(repoName "${BUILDKITE_REPO}")
BUILD_TAG="buildkite-${BUILDKITE_PIPELINE_SLUG}-${BUILDKITE_BUILD_NUMBER}"

REPO_BUILD_TAG="${REPO_NAME}/${BUILD_TAG}/"

BUILD_PACKAGES_PATH="build/packages"
TEMPLATE_TEMP_FOLDER="tmp.elastic-package.XXXXXXXXX"
JENKINS_TRIGGER_PATH=".buildkite/scripts/triggerJenkinsJob"
GOOGLE_CREDENTIALS_FILENAME="google-cloud-credentials.json"

## Signing
INFRA_SIGNING_BUCKET_NAME='internal-ci-artifacts'
INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER="${REPO_BUILD_TAG}/signed-artifacts"
INFRA_SIGNING_BUCKET_ARTIFACTS_PATH="gs://${INFRA_SIGNING_BUCKET_NAME}/${REPO_BUILD_TAG}"
INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH="gs://${INFRA_SIGNING_BUCKET_NAME}/${INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}"

## Publishing
PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH="gs://elastic-bekitzur-package-storage-internal/queue-publishing/${REPO_BUILD_TAG}"


google_cloud_auth() {
    local key_file="$1"
    gcloud gcloud auth activate-service-account --key-file ${key_file}
}

signPackage() {
    local package=${1}
    local packageZip=$(basename ${package})

    gsUtilLocation=$(mktemp -d -p . -t ${TEMPLATE_TEMP_FOLDER})

    secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}
    echo "${INTERNAL_CI_GCS_CREDENTIALS_SECRET}" > ${secretFileLocation}

    google_cloud_auth ${secretFileLocation}
    export GOOGLE_APPLICATIONS_CREDENTIALS=${secretFileLocation}

    echo "Upload package .zip file for signing"
    gsutil cp ${packageZip} ${INFRA_SIGNING_BUCKET_ARTIFACTS_PATH}

    echo "Trigger Jenkins job for signing package ${packageZip}"
    pushd ${JENKINS_TRIGGER_PATH} > /dev/null

    go run main.go \
        --jenkins-job sign \
        --package ${INFRA_SIGNING_BUCKET_ARTIFACTS_PATH}/${packageZip}

    sleep 5
    popd > /dev/null

    echo "Download signatures"
    gsutil cp ${INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH}/${packageZip}.asc ${BUILD_PACKAGES_PATH}

    echo "Rename asc to sig"
    for f in build/packages/*.asc; do
        mv "$f" "${f%.asc}.sig"
    done

    ls -l ${BUILD_PACKAGES_PATH}

    rm -r ${gsUtilLocation}
}

publishPackage() {
    local package=$1

    local packageZip=$(basename ${package})
    # create file with credentials
    gsUtilLocation=$(mktemp -d -p . -t ${TEMPLATE_TEMP_FOLDER})

    secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}
    echo "${PACKAGE_UPLOADER_GCS_CREDENTIALS_SECRET}" > ${secretFileLocation}

    google_cloud_auth ${secretFileLocation}
    export GOOGLE_APPLICATIONS_CREDENTIALS=${secretFileLocation}

    # upload files
    echo "Upload package .zip file"
    gsutil cp ${packageZip} ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}
    echo "Upload package .sig file"
    gsutil cp ${packageZip}.sig ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}

    echo "Trigger Jenkins job for publishing package ${packageZip}"
    pushd ${JENKINS_TRIGGER_PATH} > /dev/null

    go run main.go \
        --jenkins-job publish \
        --package ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip} \
        --signature ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}.sig

    sleep 5

    popd > /dev/null
    rm -r ${gsUtilLocation}
}

# download package artifact from previous step
mkdir -p ${BUILD_PACKAGES_PATH}

buildkite-agent artifact download "${BUILD_PACKAGES_PATH}/*.zip" --step build-package ${BUILD_PACKAGES_PATH}

for package in ${BUILD_PACKAGES_PATH}/*.zip; do
    echo "isAlareadyInstalled ${package}?"
    packageZip=$(basename ${package})
    if isAlreadyPublished ${packageZip} ; then
        echo "Skipping. ${packageZip} already published"
        continue
    fi

    echo "Signing package ${packageZip}"
    signPackage "${package}"

    echo "Publishing package ${packageZip}"
    publishPackage "${package}"
done
