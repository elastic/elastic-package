#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-*}}"

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  # This is a false positive scenario and tests that the test case failure is a success scenario  
  if [ "${PACKAGE_TEST_TYPE:-false_positives}" == "false_positives" ]; then
    if [ $r == 1 ]; then
        exit 0
      elif [ $r == 0 ]; then
        exit 1
    fi
  fi

  exit $r
}

trap cleanup EXIT

export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"

OLDPWD=$PWD
# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd $d
    elastic-package check -v
  )
done
cd -

# Update the stack
elastic-package stack update -v

# Boot up the stack
elastic-package stack up -d -v

elastic-package stack status

# Run package tests
eval "$(elastic-package stack shellinit)"

for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd $d
    elastic-package install -v

    # defer-cleanup is set to a short period to verify that the option is available
    elastic-package test -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage
  )
cd -
done
