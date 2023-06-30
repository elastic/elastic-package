#!/bin/bash

# exit immediately on failure, or if an undefined variable is used
set -eu

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
)

for test in ${STACK_COMMAND_TESTS[@]}; do
    echo "      - label: \":go: Running integration test: ${test}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t ${test}"
    echo "        agents:"
    echo "          provider: \"gcp\""
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
    test-check-packages-false-positives
)
for test in ${CHECK_PACKAGES_TESTS[@]}; do
    echo "      - label: \":go: Running integration test: ${test}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t ${test}"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "        artifact_paths:"
    echo "          - build/test-results/*.xml"
    echo "          - build/elastic-stack-dump/check-*/logs/*.log"
    echo "          - build/elastic-stack-dump/check-*/logs/fleet-server-internal/**/*"
    if [[ $test =~ with-kind$ ]]; then
        echo "          - build/kubectl-dump.txt"
    fi
done

pushd test/packages/parallel > /dev/null
for package in $(find . -maxdepth 1 -mindepth 1 -type d) ; do
    package_name=$(basename ${package})
    echo "      - label: \":go: Running integration test: ${package_name}\""
    echo "        key: \"integration-parallel-${package_name}\""
    echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-check-packages-parallel -p ${package_name}"
    echo "        env:"
    echo "          UPLOAD_SAFE_LOGS: 1"
    echo "        agents:"
    echo "          provider: \"gcp\""
    echo "        artifact_paths:"
    echo "          - build/test-results/*.xml"
done

popd > /dev/null

echo "      - label: \":go: Running integration test: test-build-zip\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-build-zip"
echo "        agents:"
echo "          provider: \"gcp\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/build-zip/logs/*.log"
echo "          - build/packages/*.sig"

echo "      - label: \":go: Running integration test: test-install-zip\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-install-zip"
echo "        agents:"
echo "          provider: \"gcp\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/install-zip/logs/*.log"

echo "      - label: \":go: Running integration test: test-install-zip-shellinit\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-install-zip-shellinit"
echo "        agents:"
echo "          provider: \"gcp\""
echo "        artifact_paths:"
echo "          - build/elastic-stack-dump/install-zip-shellinit/logs/*.log"

echo "      - label: \":go: Running integration test: test-profiles-command\""
echo "        command: ./.buildkite/scripts/integration_tests.sh -t test-profiles-command"
echo "        agents:"
echo "          provider: \"gcp\""
