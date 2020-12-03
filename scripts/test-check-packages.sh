#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Take down the stack
  elastic-package stack down -v

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

# Build/check packages
for d in test/packages/*/; do
  (
    cd $d
    elastic-package check -v
  )
done

# Boot up the stack
elastic-package stack up -d -v

# Run package tests
eval "$(elastic-package stack shellinit)"

for d in test/packages/*/; do
  (
    cd $d
    # defer-cleanup is set to a short period to verify that the option is available
    elastic-package test -v --report-format xUnit --report-output file --defer-cleanup 1s
  )
done