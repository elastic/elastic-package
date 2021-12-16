#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-any}}"

  if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
    # Dump kubectl details
    kubectl describe pods --all-namespaces > build/kubectl-dump.txt
    kubectl logs -l app=elastic-agent -n kube-system >> build/kubectl-dump.txt

    # Take down the kind cluster
    kind delete cluster
  fi

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd $d
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

OLDPWD=$PWD
# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
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

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
  # Boot up the kind cluster
  kind create cluster --config $PWD/scripts/kind-config.yaml
fi

# Run package tests
eval "$(elastic-package stack shellinit)"

for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd $d
    elastic-package install -v

    # defer-cleanup is set to a short period to verify that the option is available
    elastic-package test -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage
  )
cd -
done
