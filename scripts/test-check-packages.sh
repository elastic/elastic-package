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

    # Take down the kind cluster.
    # Sometimes kind has troubles with deleting the cluster, but it isn't an issue with elastic-package.
    # As it causes flaky issues on the CI side, let's ignore it.
    kind delete cluster || true
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

  # This is a false positive scenario and tests that the test case failure is a success scenario  
  if [ "${PACKAGE_TEST_TYPE:-other}" == "other" ] && [ "${PACKAGE_UNDER_TEST:-*}" == "httpjson_false_positive_asserts" ]; then
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

elastic-package stack status

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

    if [ "${PACKAGE_TEST_TYPE:-other}" == "benchmarks" ]; then
      if [ "${PACKAGE_UNDER_TEST:-*}" == "pipeline_benchmark" ]; then
        rm -rf "${OLDPWD}/build/benchmark-results"
        elastic-package benchmark pipeline -v --report-format xUnit --report-output file --fail-on-missing
        
        rm -rf "${OLDPWD}/build/benchmark-results-old"
        mv "${OLDPWD}/build/benchmark-results" "${OLDPWD}/build/benchmark-results-old"
        
        elastic-package benchmark pipeline -v --report-format json --report-output file --fail-on-missing
        
        elastic-package report --fail-on-missing benchmark \
          --new ${OLDPWD}/build/benchmark-results \
          --old ${OLDPWD}/build/benchmark-results-old \
          --threshold 1 --report-output-path="${OLDPWD}/build/benchreport"
      fi
      if [ "${PACKAGE_UNDER_TEST:-*}" == "system_benchmark" ]; then
        elastic-package benchmark system --benchmark logs-benchmark -v --defer-cleanup 1s
      fi
    else
      # defer-cleanup is set to a short period to verify that the option is available
      elastic-package test -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage
    fi
  )
cd -
done
