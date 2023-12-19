#!/bin/bash

set -euo pipefail

cleanup() {
    rm -rf "${WORKSPACE}"
}
trap cleanup exit

WORKSPACE="/tmp/bin-buildkite/"

VERSION=""
source .buildkite/scripts/install_deps.sh
source .buildkite/scripts/tooling.sh

add_bin_path
with_go

echo "--- fetching tags"
# Ensure that tags are present so goreleaser can build the changelog from the last release.
git rev-parse --is-shallow-repository
git fetch origin --tags

echo "--- running goreleaser"
# Run latest version of goreleaser
curl -sL https://git.io/goreleaser | bash
