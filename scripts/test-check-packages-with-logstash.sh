#!/bin/bash

set -euxo pipefail

function cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-*}}"

  # Delete the logstash profile
  elastic-package profiles delete logstash -v

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-with-logstash}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"

# Create a logstash profile and use it
elastic-package profiles create logstash -v
elastic-package profiles use logstash

# Rename the config.yml.example to config.yml
mv ~/.elastic-package/profiles/logstash/config.yml.example ~/.elastic-package/profiles/logstash/config.yml -v

# Add config to enable logstash
echo "stack.logstash_enabled: true" >> ~/.elastic-package/profiles/logstash/config.yml

# Update the stack
elastic-package stack update -v

# Boot up the stack
elastic-package stack up -d -v

elastic-package stack status

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE:-with-logstash}/${PACKAGE_UNDER_TEST:-*}/; do
  cd $d
  elastic-package test -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage
done
