#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

cleanup_cloud_stale() {
    local exit_code=$?

    cd "$WORKSPACE"
    rm -f "${AWS_RESOURCES_FILE}"
    rm -f "${AWS_REDSHIFT_RESOURCES_FILE}"

    exit "$exit_code"
}

trap cleanup_cloud_stale EXIT

set -euo pipefail

AWS_RESOURCES_FILE="aws.resources.txt"
AWS_REDSHIFT_RESOURCES_FILE="redshift_clusters.json"

DRY_RUN="$(buildkite-agent meta-data get DRY_RUN --default "${DRY_RUN:-"true"}")"
RESOURCE_RETENTION_PERIOD="$(buildkite-agent meta-data get RESOURCE_RETENTION_PERIOD --default "${RESOURCE_RETENTION_PERIOD:-"24 hours"}")"
DELETE_RESOURCES_BEFORE_DATE=$(date -Is -d "${RESOURCE_RETENTION_PERIOD} ago")
export DELETE_RESOURCES_BEFORE_DATE

CLOUD_REAPER_IMAGE="${DOCKER_REGISTRY}/observability-ci/cloud-reaper:0.3.0"


resources_to_delete=0
resources_failed_to_delete=0

COMMAND="validate"
redshift_message=""
if [[ "${DRY_RUN}" != "true" ]]; then
    # TODO: to be changed to "destroy --confirm" once it can be tested
    # that filters work as expected
    COMMAND="destroy --confirm"
    redshift_message=" - stale redshift clusters will be deleted"
else
    COMMAND="plan"
fi

buildkite-agent annotate \
  "[${BUILDKITE_STEP_KEY}] Running DRY_RUN (${DRY_RUN}) using cloud-reaper command \"${COMMAND}\"${redshift_message}" \
  --context "ctx-cloud-reaper-info" \
  --style "info"

any_resources_to_delete() {
    local file=$1
    local number=0
    # First three lines are like:
    # ⇒ Loading configuration...
    # ✓ Succeeded to load configuration
    # Scanning resources... ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100% 0:00:00
    
    if [[ "${DRY_RUN}" == false ]] ; then
        # if DRY_RUN is false, it needs to check whether or not there are failures deleting them
        # via any_resources_failed_to_delete function
        return 1
    fi
    number=$(tail -n +4 "${file}" | wc -l)
    if [ "${number}" -eq 0 ]; then
        return 1
    fi
    return 0
}

any_resources_failed_to_delete() {
    local file=$1
    # In the case, there is nothing to delete, there is one more line:
    # ⇒ Nothing to destroy !
    if [[ "${DRY_RUN}" == false ]] ; then
        if tail -n 1 "${file}" | grep -q "Nothing to destroy" ; then
            return 1
        fi
        # cloud-reaper should show FAILED in case there is some error deleting resources
        # if everything runs successfully, it is shown SUCCEEDED
        # successful example:
        # Scanning resources... ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100% 0:00:00
        # ObjectStorageBucket( name=elastic-package-sinkhole-http-bucket-771d12318 )
        # ObjectStorageBucket( name=elastic-package-spectrum-event-bucket-44ec66812 )
        # ObjectStorageBucket( name=elastic-package-sinkhole-http-bucket-771d12318 ):
        # SUCCEEDED
        # ObjectStorageBucket( name=elastic-package-spectrum-event-bucket-44ec66812 ):
        # SUCCEEDED
        # Destroying resources... ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100% 0:00:00
        if grep -q FAILED "${file}" ; then
            return 0
        fi
    fi
    return 1
}

# As long as cloud reaper does not support OIDC authentication.
create_aws_ephemeral_user() {
    # Generate a unique name for the ephemeral IAM user.
    EPHEMERAL_USER="ephemeral-admin-$(date +%s)"
    echo "Creating IAM user: ${EPHEMERAL_USER}"
    aws iam create-user --user-name "${EPHEMERAL_USER}" \
        --tags Key=ephemeral,Value=true Key=division,Value=engineering Key=org,Value=obs Key=environment,Value=ci Key=repo,Value=elastic-package Key=created_at,Value="$(date -Is)"

    echo "Attaching AdministratorAccess policy to ${EPHEMERAL_USER}..."
    aws iam attach-user-policy --user-name "${EPHEMERAL_USER}" --policy-arn arn:aws:iam::aws:policy/AdministratorAccess

    echo "Creating access keys for ${EPHEMERAL_USER}..."
    creds_json=$(aws iam create-access-key --user-name "${EPHEMERAL_USER}")
    AWS_ACCESS_KEY_ID_EPHEMERAL=$(echo "$creds_json" | jq -r '.AccessKey.AccessKeyId')
    AWS_SECRET_ACCESS_KEY_EPHEMERAL=$(echo "$creds_json" | jq -r '.AccessKey.SecretAccessKey')
    export EPHEMERAL_USER AWS_ACCESS_KEY_ID_EPHEMERAL AWS_SECRET_ACCESS_KEY_EPHEMERAL
}

# Define cleanup function to delete the ephemeral IAM user regardless of script outcome.
cleanup_ephemeral_user() {
    echo "Cleaning up ephemeral IAM user: ${EPHEMERAL_USER}"
    aws iam detach-user-policy --user-name "${EPHEMERAL_USER}" --policy-arn arn:aws:iam::aws:policy/AdministratorAccess
    key_id=$(echo "$creds_json" | jq -r '.AccessKey.AccessKeyId')
    aws iam delete-access-key --user-name "${EPHEMERAL_USER}" --access-key-id "${key_id}"
    aws iam delete-user --user-name "${EPHEMERAL_USER}"
    echo "Ephemeral IAM user ${EPHEMERAL_USER} deleted."
    unset EPHEMERAL_USER AWS_ACCESS_KEY_ID_EPHEMERAL AWS_SECRET_ACCESS_KEY_EPHEMERAL
}
trap cleanup_ephemeral_user EXIT

cloud_reaper_aws() {
    echo "--- Configuring ephemeral user"
    create_aws_ephemeral_user

    echo "--- Validating configuration"
    docker run --rm -v "$(pwd)/.buildkite/configs/cleanup.aws.yml":/etc/cloud-reaper/config.yml \
      -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID_EPHEMERAL" \
      -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY_EPHEMERAL" \
      -e ACCOUNT_PROJECT="observability-ci" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      "${CLOUD_REAPER_IMAGE}" \
        cloud-reaper \
          --debug \
          --config /etc/cloud-reaper/config.yml \
          validate

    if [[ "${DRY_RUN}" == "true" ]]; then
        echo "--- Scanning resources (DRY_RUN ${DRY_RUN})"
    else 
        echo "--- Scanning and deleting resources (DRY_RUN ${DRY_RUN})"
    fi
    docker run --rm -v "$(pwd)/.buildkite/configs/cleanup.aws.yml":/etc/cloud-reaper/config.yml \
      -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID_EPHEMERAL" \
      -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY_EPHEMERAL" \
      -e ACCOUNT_PROJECT="observability-ci" \
      -e CREATION_DATE="${DELETE_RESOURCES_BEFORE_DATE}" \
      "${CLOUD_REAPER_IMAGE}" \
        cloud-reaper \
          --config /etc/cloud-reaper/config.yml \
          ${COMMAND} | tee "${AWS_RESOURCES_FILE}"
}

echo "--- Installing awscli"
with_aws_cli

echo "--- Cleaning up AWS resources older than ${DELETE_RESOURCES_BEFORE_DATE}..."
cloud_reaper_aws

if any_resources_to_delete "${AWS_RESOURCES_FILE}" ; then
    echo "Pending AWS resources"
    resources_to_delete=1
fi

if any_resources_failed_to_delete "${AWS_RESOURCES_FILE}" ; then
    echo "Failed to delete at least one resource. Check output."
    resources_failed_to_delete=1
fi

if [ "${resources_to_delete}" -eq 1 ]; then
    message="There are resources to be deleted"
    echo "${message}"
    if running_on_buildkite ; then
         buildkite-agent annotate \
             "${message}" \
             --context "ctx-cloud-reaper-error-pending" \
             --style "error"
    fi
fi

if [ "${resources_failed_to_delete}" -eq 1 ]; then
    message="There are resources that could not be deleted. Check the logs for details."
    echo "${message}"
    if running_on_buildkite ; then
         buildkite-agent annotate \
             "${message}" \
             --context "ctx-cloud-reaper-error-deleting" \
             --style "error"
    fi
fi

echo "--- Cleaning up other AWS resources older than ${DELETE_RESOURCES_BEFORE_DATE}"

export AWS_DEFAULT_REGION=us-east-1
# Avoid to send the output of the CLI to a pager
export AWS_PAGER=""

echo "--- Checking if any Redshift cluster still created"
aws redshift describe-clusters \
    --tag-keys "environment" \
    --tag-values "ci" > "${AWS_REDSHIFT_RESOURCES_FILE}"

clusters_num=$(jq -rc '.Clusters | length' "${AWS_REDSHIFT_RESOURCES_FILE}")

echo "Number of clusters found: ${clusters_num}"

redshift_clusters_to_delete=0
while read -r i ; do
    identifier=$(echo "$i" | jq -rc ".ClusterIdentifier")
    # tags
    repo=$(echo "$i" | jq -rc '.Tags[] | select(.Key == "repo").Value')
    environment=$(echo "$i" | jq -rc '.Tags[] | select(.Key == "environment").Value')
    # creation time tag in milliseconds
    createdAt=$(echo "$i" | jq -rc '.Tags[] | select(.Key == "created_date").Value')
    # epoch in milliseconds minus retention period
    thresholdEpoch=$(date -d "${RESOURCE_RETENTION_PERIOD} ago" +"%s%3N")

    if [[ ! "${identifier}" =~ ^elastic-package-test- ]]; then
        echo "Skip cluster ${identifier}, do not match required identifiers."
        continue
    fi

    if [[ "${repo}" != "integrations" && "${repo}" != "elastic-package" ]]; then
        echo "Skip cluster ${identifier}, not from the expected repo: ${repo}."
        continue
    fi

    if [[ "${environment}" != "ci" ]]; then
        echo "Skip cluster ${identifier}, not from the expected environment: ${environment}."
        continue
    fi

    if [ "${createdAt}" -gt "${thresholdEpoch}" ]; then
        echo "Skip cluster $identifier. It was created < ${RESOURCE_RETENTION_PERIOD} ago"
        continue
    fi

    echo "To be deleted cluster: $identifier. It was created > ${RESOURCE_RETENTION_PERIOD} ago"
    if [ "${DRY_RUN}" != "false" ]; then
        redshift_clusters_to_delete=1
        continue
    fi

    echo "Deleting: $identifier. It was created > ${RESOURCE_RETENTION_PERIOD} ago"
    if ! aws redshift delete-cluster \
      --cluster-identifier "${identifier}" \
      --skip-final-cluster-snapshot \
      --output json \
      --query "Cluster.{ClusterStatus:ClusterStatus,ClusterIdentifier:ClusterIdentifier}" ; then

        echo "Failed delete-cluster"
        buildkite-agent annotate \
            "Deleted redshift cluster: ${identifier}" \
            --context "ctx-aws-readshift-deleted-error-${identifier}" \
            --style "error"

        redshift_clusters_to_delete=1
    else
        echo "Done."
        # if deletion works, no need to mark this one as to be deleted
        buildkite-agent annotate \
            "Deleted redshift cluster: ${identifier}" \
            --context "ctx-aws-readshift-deleted-${identifier}" \
            --style "success"
    fi
done <<< "$(jq -c '.Clusters[]' "${AWS_REDSHIFT_RESOURCES_FILE}")"

if [ "${redshift_clusters_to_delete}" -eq 1 ]; then
    resources_to_delete=1
    message="There are redshift resources to be deleted"
    echo "${message}"
    if running_on_buildkite ; then
         buildkite-agent annotate \
             "${message}" \
             --context "ctx-aws-redshift-error-pending" \
             --style "error"
    fi
fi

# TODO: List and delete the required resources using aws cli or using cloud-reaper tool
echo "--- TODO: Cleaning up IAM roles"
echo "--- TODO: Cleaning up IAM policies"
echo "--- TODO: Cleaning up Schedulers"

if [ "${resources_to_delete}" -eq 1 ]; then
    exit 1
fi

if [ "${resources_failed_to_delete}" -eq 1 ]; then
    exit 1
fi
