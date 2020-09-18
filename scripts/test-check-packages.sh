#!/bin/bash

set -euxo pipefail

elastic-package stack up -d -v

eval "$(elastic-package stack shellinit)"

for d in test/packages/*/; do
  (
    cd $d
    elastic-package check -v
    elastic-package test -v
  )
done

elastic-package stack down -v
