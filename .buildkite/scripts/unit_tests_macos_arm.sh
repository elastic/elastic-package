#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

set -euo pipefail

add_bin_path

echo "--- install go"
with_go

make test-go-ci

