#!/bin/bash

set -euo pipefail

WORKSPACE="$(pwd)"
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
# curl -sL https://git.io/goreleaser | bash

TAR_FILE="/tmp/goreleaser.tar.gz"
RELEASES_URL="https://github.com/goreleaser/goreleaser/releases"
# test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"
TARGET_DIR="${WORKSPACE}/bin"

last_version() {
  curl -sL -o /dev/null -w %{url_effective} "$RELEASES_URL/latest" |
    rev |
    cut -f1 -d'/'|
    rev
}

download() {
  test -z "$VERSION" && VERSION="$(last_version)"
  test -z "$VERSION" && {
    echo "Unable to get goreleaser version." >&2
    exit 1
  }
  rm -f "$TAR_FILE"
  curl -s -L -o "$TAR_FILE" \
    "$RELEASES_URL/download/$VERSION/goreleaser_$(uname -s)_$(uname -m).tar.gz"
}

download
tar -xf "$TAR_FILE" -C "${TARGET_DIR}"

rm ${TAR_FILE}
chmod u+x ${TARGET_DIR}/goreleaser

# build
goreleaser build

# release skip
goreleaser release --skip-publish
