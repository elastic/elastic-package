#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Clean used resources
  for d in test/packages/*/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

OLDPWD=$PWD

# Build packages
for d in test/packages/*/; do
  (
    cd $d
    elastic-package build --zip -v
  )
done
cd -

# Sign using Infra pipeline

# TODO upload to S3 bucket
# TODO call infra pipeline
# TODO verify signatures