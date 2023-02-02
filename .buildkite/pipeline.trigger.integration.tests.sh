#!/bin/bash

# exit immediately on failure, or if an undefined variable is used
set -eu

# begin the pipeline.yml file
echo "steps:"

# Integration tests related stack command
STACK_COMMAND_TESTS = (
  test-stack-command-default
  test-stack-command-oldest
)

for test in ${STACK_COMMAND_TESTS[@]}; do
    echo "  - label: :go: Running integration test: ${test}"
    echo "    command: ./buildkite/scripts/integration_tests.sh -t ${test}"
    echo "    agents:"
    echo "      provider: \"gcp\""
    echo "    artifact_paths:"
    echo "      - build/elastic-stack-dump/stack/*/logs/*.log"
    echo "      - build/elastic-stack-dump/stack/*/logs/fleet-server-internal/*.log"
    echo "      - build/elastic-stack-status/*/*"
done
