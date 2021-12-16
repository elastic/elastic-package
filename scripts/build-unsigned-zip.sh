#!/bin/bash

set -euxo pipefail

OLDPWD=$PWD

# Build packages
for d in test/packages/*/*/; do
  (
    cd $d
    elastic-package build --zip -v
  )
done
cd -