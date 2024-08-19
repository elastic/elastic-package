#!/usr/bin/env bash

source .buildkite/scripts/install_deps.sh

set -euo pipefail

add_bin_path

echo "--- install go"
with_go

echo "--- install docker"
with_docker

echo "--- install docker-compose plugin"
with_docker_compose_plugin

make test-go-ci

