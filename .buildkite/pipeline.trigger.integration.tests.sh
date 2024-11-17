#!/bin/bash

# exit immediately on failure, or if an undefined variable is used
set -eu

echoerr() {
    echo "$@" 1>&2
}

# begin the pipeline.yml file
echo "steps:"
echo "  - group: \":terminal: Integration test suite\""
echo "    key: \"integration-tests\""
echo "    steps:"

# Integration tests related stack command
STACK_COMMAND_TESTS=(
    test-stack-command-default
    test-stack-command-oldest
    test-stack-command-7x
    test-stack-command-86
    test-stack-command-8x
    test-stack-command-9x
    test-stack-command-with-apm-server
)

for test in "${STACK_COMMAND_TESTS[@]}"; do
    test_name=${test#"test-"}
    echo "      - label: \":go: Integration test: ${test_name}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t ${test}"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
    echo "        artifact_paths:"
    echo "          - build/elastic-stack-dump/stack/*/logs/*.log"
    echo "          - build/elastic-stack-dump/stack/*/logs/fleet-server-internal/**/*"
    echo "          - build/elastic-stack-status/*/*"
done

CHECK_PACKAGES_TESTS=(
    test-check-packages-other
    test-check-packages-with-kind
    test-check-packages-with-custom-agent
    test-check-packages-benchmarks
    test-check-packages-with-logstash
)
for independent_agent in false true ; do
for test in "${CHECK_PACKAGES_TESTS[@]}"; do
    label_suffix=""
    if [[ "$independent_agent" == "false" ]]; then
        label_suffix=" (stack agent)"
    fi
    test_name=${test#"test-check-packages-"}
    echo "      - label: \":go: Integration test: ${test_name}${label_suffix}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t ${test}"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
    echo "        artifact_paths:"
    echo "          - build/test-results/*.xml"
    echo "          - build/elastic-stack-dump/check-*/logs/*.log"
    echo "          - build/elastic-stack-dump/check-*/logs/fleet-server-internal/**/*"
    echo "          - build/test-coverage/coverage-*.xml" # these files should not be used to compute the final coverage of elastic-package
    if [[ $test =~ with-kind$ ]]; then
        echo "          - build/kubectl-dump.txt"
    fi
    if [[ "${independent_agent}" == "false" ]]; then
        echo "        env:"
        echo "          ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT: ${independent_agent}"
    fi
done
done

pushd test/packages/false_positives > /dev/null
for package in $(find . -maxdepth 1 -mindepth 1 -type d) ; do
    package_name=$(basename "${package}")
    echo "      - label: \":go: Integration test (false positive): ${package_name}\""
    echo "        key: \"integration-false_positives-${package_name}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-check-packages-false-positives -p ${package_name}"
    echo "        env:"
    echo "          UPLOAD_SAFE_LOGS: 1"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
    echo "        artifact_paths:"
    echo "          - build/test-results/*.xml"
    echo "          - build/test-results/*.xml.expected-errors.txt"  # these files are uploaded in case it is needed to review the xUnit files in case of CI reports success the step
    echo "          - build/test-coverage/coverage-*.xml" # these files should not be used to compute the final coverage of elastic-package
done
popd > /dev/null

pushd test/packages/parallel > /dev/null
for independent_agent in false true; do
for package in $(find . -maxdepth 1 -mindepth 1 -type d) ; do
    label_suffix=""
    if [[ "$independent_agent" == "false" ]]; then
        label_suffix=" (stack agent)"
    fi
    package_name=$(basename "${package}")
    if [[ "$independent_agent" == "false" && "$package_name" == "oracle" ]]; then
        echoerr "Package \"${package_name}\" skipped: not supported with Elastic Agent running in the stack (missing required software)."
        continue
    fi

    if [[ "$independent_agent" == "false" && "$package_name" == "auditd_manager" ]]; then
        echoerr "Package \"${package_name}\" skipped: not supported with Elastic Agent running in the stack (missing capabilities)."
        continue
    fi

    if [[ "$independent_agent" == "false" && "$package_name" == "custom_entrypoint" ]]; then
        echoerr "Package \"${package_name}\" skipped: not supported with Elastic Agent running in the stack (missing required files deployed in provisioning)."
        continue
    fi

    echo "      - label: \":go: Integration test: ${package_name}${label_suffix}\""
    echo "        key: \"integration-parallel-${package_name}-agent-${independent_agent}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-check-packages-parallel -p ${package_name}"
    echo "        env:"
    echo "          UPLOAD_SAFE_LOGS: 1"
    if [[ "${independent_agent}" == "false" ]]; then
        echo "          ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT: ${independent_agent}"
    fi
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
    echo "        artifact_paths:"
    echo "          - build/test-results/*.xml"
    echo "          - build/test-coverage/coverage-*.xml" # these files should not be used to compute the final coverage of elastic-package
done
done
popd > /dev/null

# TODO: Missing docker & docker-compose in MACOS ARM agent image, skip installation of packages in the meantime.
# If docker and docker-compose are available for this platform/architecture, it could be added a step to test the stack commands (or even replace this one).
echo "      - label: \":macos: :go: Integration test: build-zip\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-build-zip"
echo "        agents:"
echo "          provider: \"orka\""
echo "          imagePrefix: \"${MACOS_ARM_AGENT_IMAGE}\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/build-zip/logs/*.log"
echo "          - build/packages/*.sig"

echo "      - label: \":go: Integration test: build-install-zip\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-build-install-zip"
echo "        agents:"
echo "          provider: \"gcp\""
echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/build-zip/logs/*.log"
echo "          - build/packages/*.sig"

echo "      - label: \":go: Integration test: build-install-zip-file\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-build-install-zip-file"
echo "        agents:"
echo "          provider: \"gcp\""
echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/install-zip/logs/*.log"

echo "      - label: \":go: Integration test: build-install-zip-file-shellinit\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-build-install-zip-file-shellinit"
echo "        agents:"
echo "          provider: \"gcp\""
echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/install-zip-shellinit/logs/*.log"

for independent_agent in false true; do
    label_suffix=""
    if [[ "$independent_agent" == "false" ]]; then
        label_suffix=" (stack agent)"
    fi
    echo "      - label: \":go: Integration test: system-flags${label_suffix}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-system-test-flags"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "          image: \"${UBUNTU_X86_64_AGENT_IMAGE}\""
    echo "        env:"
    echo "          ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT: ${independent_agent}"
done

echo "      - label: \":go: Integration test: profiles-command\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-profiles-command"
echo "        env:"
echo "          DOCKER_COMPOSE_VERSION: \"false\""
echo "          DOCKER_VERSION: \"false\""
echo "        agents:"
echo "          image: \"${LINUX_GOLANG_AGENT_IMAGE}\""
echo "          cpu: \"8\""
echo "          memory: \"4G\""

echo "      - label: \":go: Integration test: check-update-version\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-check-update-version"
echo "        env:"
echo "          DEFAULT_VERSION_TAG: v0.80.0"
echo "          DOCKER_COMPOSE_VERSION: \"false\""
echo "          DOCKER_VERSION: \"false\""
echo "        agents:"
echo "          image: \"${LINUX_GOLANG_AGENT_IMAGE}\""
echo "          cpu: \"8\""
echo "          memory: \"4G\""
