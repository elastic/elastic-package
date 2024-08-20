#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

set -euo pipefail

add_bin_path

echo "--- install go"
with_go

echo "--- Running unit tests"
make test-go-ci

# Force a different filename from linux step, so it can be processed
# by the JUnit buildkite step
mv build/test-results/TEST-unit.xml build/test-results/TEST-unit-darwin.xml

