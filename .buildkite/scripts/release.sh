#!/bin/bash

set -euo pipefail

echo "--- fetching tags"
# Ensure that tags are present so goreleaser can build the changelog from the last release.
git rev-parse --is-shallow-repository
git fetch origin --tags

echo "--- running goreleaser"
# Run latest version of goreleaser
# TODO Remove check command once this stage is removed from Jenkinsfile
# curl -sL https://git.io/goreleaser | bash
curl -sL https://git.io/goreleaser | bash -s -- check

