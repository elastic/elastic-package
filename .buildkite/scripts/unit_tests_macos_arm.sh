#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

set -euo pipefail

add_bin_path

echo "--- Install go"
with_go

echo "--- Running unit tests"
make test-go-ci

