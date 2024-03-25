#!/bin/bash

set -euxo pipefail

cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v \
      --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-any}}"

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

  if [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ]; then
    # Delete the logstash profile
    elastic-package profiles delete logstash -v
  fi

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd "$d"
      elastic-package clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_ENABLE_INDEPENDENT_AGENT=${ELASTIC_PACKAGE_ENABLE_INDEPENDENT_AGENT:-"false"}
export ELASTIC_PACKAGE_INDEPENDENT_AGENT
ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

OLDPWD=$PWD
# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd "$d"
    elastic-package check -v
  )
done
cd -

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ]; then
  # Create a logstash profile and use it
  elastic-package profiles create logstash -v
  elastic-package profiles use logstash

  # Rename the config.yml.example to config.yml
  mv ~/.elastic-package/profiles/logstash/config.yml.example ~/.elastic-package/profiles/logstash/config.yml
  
  # Append config to enable logstash
  echo "stack.logstash_enabled: true" >> ~/.elastic-package/profiles/logstash/config.yml
fi

# Update the stack
elastic-package stack update -v

# Boot up the stack
elastic-package stack up -d -v

elastic-package stack status

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
  # Boot up the kind cluster
  kind create cluster --config "$PWD/scripts/kind-config.yaml"
fi

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd "$d"

    if [ "${PACKAGE_TEST_TYPE:-other}" == "benchmarks" ]; then
      # It is not used PACKAGE_UNDER_TEST, so all benchmark packages are run in the same loop
      package_to_test=$(basename "${d}")
      if [ "${package_to_test}" == "pipeline_benchmark" ]; then
        rm -rf "${OLDPWD}/build/benchmark-results"
        elastic-package benchmark pipeline -v --report-format xUnit --report-output file --fail-on-missing

        rm -rf "${OLDPWD}/build/benchmark-results-old"
        mv "${OLDPWD}/build/benchmark-results" "${OLDPWD}/build/benchmark-results-old"

        elastic-package benchmark pipeline -v --report-format json --report-output file --fail-on-missing

        elastic-package report --fail-on-missing benchmark \
          --new "${OLDPWD}/build/benchmark-results" \
          --old "${OLDPWD}/build/benchmark-results-old" \
          --threshold 1 --report-output-path="${OLDPWD}/build/benchreport"
      fi
      if [ "${package_to_test}" == "system_benchmark" ]; then
        elastic-package benchmark system --benchmark logs-benchmark -v --defer-cleanup 1s
      fi
    elif [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ] && [ "${PACKAGE_UNDER_TEST:-*}" == "system_benchmark" ]; then
        elastic-package benchmark system --benchmark logs-benchmark -v --defer-cleanup 1s
    else
      # defer-cleanup is set to a short period to verify that the option is available
      elastic-package test -v \
          --report-format xUnit \
          --report-output file \
          --defer-cleanup 1s \
          --test-coverage \
          --coverage-format=generic
    fi
  )
cd -
done
