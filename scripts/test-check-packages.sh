#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?
  elastic-package stack down -v
  exit $r
}

trap cleanup EXIT

elastic-package stack up -d -v

eval "$(elastic-package stack shellinit)"

for d in test/packages/*/; do
  (
    cd $d
    elastic-package check -v
    elastic-package test -v -r xUnit
  )
done