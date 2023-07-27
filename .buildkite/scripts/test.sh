#!/bin/bash

set -euxo pipefail

echo "${TEST_MULTILINE_SECRET}" > other_file

cat other_file
