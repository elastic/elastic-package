#!/bin/bash
set -euo pipefail

WORKSPACE="$(pwd)"
TMP_FOLDER_TEMPLATE_BASE="tmp.elastic-package"

source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

logout() {
    local error_code=$?
    if [ $error_code != 0 ] ; then
        if [ -f ${GOOGLE_APPLICATION_CREDENTIALS} ]; then
            google_cloud_logout_active_account
        fi
    fi
    exit $error_code
}
# trap needed to ensure that no account is activated in case of failure
trap logout EXIT

is_already_published() {
    local packageZip=$1

    if curl -s --head https://package-storage.elastic.co/artifacts/packages/${packageZip} | grep -q "HTTP/2 200" ; then
        echo "- Already published ${packageZip}"
        return 0
    fi
    echo "- Not published ${packageZip}"
    return 1
}

echo "Checking gsutil command..."
if ! command -v gsutil &> /dev/null ; then
    echo "⚠️  gsutil is not installed"
    exit 1
fi


REPO_NAME=$(repo_name "${BUILDKITE_REPO}")
BUILD_TAG="buildkite-${BUILDKITE_PIPELINE_SLUG}-${BUILDKITE_BUILD_NUMBER}"

REPO_BUILD_TAG="${REPO_NAME}/${BUILD_TAG}"

BUILD_PACKAGES_PATH="build/packages"
TMP_FOLDER_TEMPLATE="${TMP_FOLDER_TEMPLATE_BASE}.XXXXXXXXX"
JENKINS_TRIGGER_PATH=".buildkite/scripts/triggerJenkinsJob"
GOOGLE_CREDENTIALS_FILENAME="google-cloud-credentials.json"

## Signing
INFRA_SIGNING_BUCKET_NAME='internal-ci-artifacts'
INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER="${REPO_BUILD_TAG}/signed-artifacts"
INFRA_SIGNING_BUCKET_ARTIFACTS_PATH="gs://${INFRA_SIGNING_BUCKET_NAME}/${REPO_BUILD_TAG}"
INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH="gs://${INFRA_SIGNING_BUCKET_NAME}/${INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_SUBFOLDER}"

## Publishing
PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH="gs://elastic-bekitzur-package-storage-internal/queue-publishing/${REPO_BUILD_TAG}"


google_cloud_auth_signing() {
    local gsUtilLocation=$(mktemp -d -p . -t ${TMP_FOLDER_TEMPLATE})

    local secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}
    echo "${SIGNING_PACKAGES_GCS_CREDENTIALS_SECRET}" > ${secretFileLocation}

    google_cloud_auth "${secretFileLocation}"

    echo "${gsUtilLocation}"
}

google_cloud_auth_publishing() {
    local gsUtilLocation=$(mktemp -d -p . -t ${TMP_FOLDER_TEMPLATE})

    local secretFileLocation=${gsUtilLocation}/${GOOGLE_CREDENTIALS_FILENAME}
    echo "${PACKAGE_UPLOADER_GCS_CREDENTIALS_SECRET}" > ${secretFileLocation}

    google_cloud_auth "${secretFileLocation}"

    echo "${gsUtilLocation}"
}

sign_package() {
    local package=${1}
    local packageZip=$(basename ${package})

    local gsUtilLocation=$(google_cloud_auth_signing)

    # upload zip package (trailing forward slashes are required)
    echo "Upload package .zip file for signing ${package} to ${INFRA_SIGNING_BUCKET_ARTIFACTS_PATH}"
    gsutil cp ${package} "${INFRA_SIGNING_BUCKET_ARTIFACTS_PATH}/"

    echo "Trigger Jenkins job for signing package ${packageZip}"
    pushd ${JENKINS_TRIGGER_PATH} > /dev/null

    go run main.go \
        --jenkins-job sign \
        --folder ${INFRA_SIGNING_BUCKET_ARTIFACTS_PATH}

    sleep 5
    popd > /dev/null

    echo "Download signatures"
    gsutil cp "${INFRA_SIGNING_BUCKET_SIGNED_ARTIFACTS_PATH}/${packageZip}.asc" "${BUILD_PACKAGES_PATH}"

    echo "Rename asc to sig"
    for f in $(ls ${BUILD_PACKAGES_PATH}/*.asc); do
        mv "$f" "${f%.asc}.sig"
    done

    ls -l "${BUILD_PACKAGES_PATH}"

    google_cloud_logout_active_account

    echo "Removing temporal location ${gsUtilLocation}"
    rm -f "${gsUtilLocation}"
}

publish_package() {
    local package=$1
    local packageZip=$(basename ${package})

    # create file with credentials
    local gsUtilLocation=$(google_cloud_auth_publishing)

    # upload files (trailing forward slashes are required)
    echo "Upload package .zip file ${package} to ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}"
    gsutil cp ${package} "${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/"
    echo "Upload package .sig file ${package}.sig to ${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}"
    gsutil cp ${package}.sig "${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/"

    echo "Trigger Jenkins job for publishing package ${packageZip}"
    pushd ${JENKINS_TRIGGER_PATH} > /dev/null

    go run main.go \
        --jenkins-job publish \
        --package "${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}" \
        --signature "${PACKAGE_STORAGE_INTERNAL_BUCKET_QUEUE_PUBLISHING_PATH}/${packageZip}.sig"

    sleep 5

    popd > /dev/null

    google_cloud_logout_active_account

    echo "Removing temporal location ${gsUtilLocation}"
    rm -f "${gsUtilLocation}"
}

add_bin_path

# Required to trigger Jenkins job
with_go

# download package artifact from previous step
mkdir -p "${BUILD_PACKAGES_PATH}"

buildkite-agent artifact download "${BUILD_PACKAGES_PATH}/*.zip" --step build-package .
echo "Show artifacts downloaded from previous step ${BUILD_PACKAGES_PATH}"
ls -l "${BUILD_PACKAGES_PATH}"

for package in $(ls ${BUILD_PACKAGES_PATH}/*.zip); do
    echo "isAlreadyInstalled ${package}?"
    packageZip=$(basename ${package})
    if is_already_published ${packageZip} ; then
        echo "Skipping. ${packageZip} already published"
        continue
    fi

    echo "Signing package ${packageZip}"
    sign_package "${package}"

    echo "Publishing package ${packageZip}"
    publish_package "${package}"
done
