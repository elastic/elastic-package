#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_parameters.sh"

set -euxo pipefail

cleanup() {
  r=$?
  echo "~~~ elastic-package cleanup"

  if [[ "${SERVERLESS}" == "true" || "${ELASTIC_PACKAGE_STARTED}" == "1" ]]; then
    # Dump stack logs
    elastic-package stack dump -v \
        --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-any}}"
  fi

  if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
    # Dump kubectl details
    kubectl describe pods --all-namespaces > build/kubectl-dump.txt
    kubectl logs -l app=elastic-agent -n kube-system >> build/kubectl-dump.txt

    # Take down the kind cluster.
    # Sometimes kind has troubles with deleting the cluster, but it isn't an issue with elastic-package.
    # As it causes flaky issues on the CI side, let's ignore it.
    kind delete cluster || true
  fi

  # In case it is tested with Elastic serverless, there should be just one Elastic stack
  # started to test all packages. In our CI, this Elastic serverless stack is started 
  # at the beginning of the pipeline and must be running for all packages without stopping it between
  # packages.
  if [[ "$SERVERLESS" != "true" && "${ELASTIC_PACKAGE_STARTED}" == 1 ]]; then
      # Take down the stack
      elastic-package stack down -v
  fi

  if [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ]; then
    # Delete the logstash profile
    elastic-package profiles delete logstash -v
  fi

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH
export SERVERLESS=${SERVERLESS:-"false"}

# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  echo "--- Checking package ${d}"
  elastic-package check -C "$d" -v
done

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ]; then
  echo "--- Create logstash profile"

  # Create a logstash profile and use it
  elastic-package profiles create logstash -v
  elastic-package profiles use logstash

  # Rename the config.yml.example to config.yml
  mv ~/.elastic-package/profiles/logstash/config.yml.example ~/.elastic-package/profiles/logstash/config.yml
  
  # Append config to enable logstash
  echo "stack.logstash_enabled: true" >> ~/.elastic-package/profiles/logstash/config.yml
fi

# In case it is tested with Elastic serverless, there should be just one Elastic stack
# started to test all packages. In our CI, this Elastic serverless stack is started 
# at the beginning of the pipeline and must be running for all packages.
ELASTIC_PACKAGE_STARTED=0
if [[ "${SERVERLESS}" != "true" ]]; then
  echo "--- Prepare Elastic stack"
  stack_args=$(stack_version_args) # --version <version>

  # Update the stack
  elastic-package stack update -v ${stack_args}

  # NOTE: if any provider argument is defined, the stack must be shutdown first to ensure
  # that all parameters are taken into account by the services
  stack_args="${stack_args} $(stack_provider_args)" # -U <setting=1,settings=2>

  # Boot up the stack
  elastic-package stack up -d -v ${stack_args}

  ELASTIC_PACKAGE_STARTED=1

  elastic-package stack status
fi

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
  # Boot up the kind cluster
  echo "--- Create kind cluster"
  kind create cluster --config "$PWD/scripts/kind-config.yaml" --image "kindest/node:${K8S_VERSION}"
fi

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  package_to_test=$(basename "${d}")

  if [ "${PACKAGE_TEST_TYPE:-other}" == "benchmarks" ]; then
    # It is not used PACKAGE_UNDER_TEST, so all benchmark packages are run in the same loop
    if [ "${package_to_test}" == "pipeline_benchmark" ]; then
      echo "--- Run pipeline benchmarks and report for package ${package_to_test}"

      rm -rf "${PWD}/build/benchmark-results"
      elastic-package benchmark pipeline -C "$d" -v --report-format xUnit --report-output file --fail-on-missing

      rm -rf "${PWD}/build/benchmark-results-old"
      mv "${PWD}/build/benchmark-results" "${PWD}/build/benchmark-results-old"

      elastic-package benchmark pipeline -C "$d" -v --report-format json --report-output file --fail-on-missing

      elastic-package report -C "$d" --fail-on-missing benchmark \
        --new "${PWD}/build/benchmark-results" \
        --old "${PWD}/build/benchmark-results-old" \
        --threshold 1 --report-output-path="${PWD}/build/benchreport"
    fi
    if [ "${package_to_test}" == "system_benchmark" ]; then
      echo "--- Run system benchmarks and report for package ${package_to_test}"

      elastic-package benchmark system -C "$d" --benchmark logs-benchmark -v --defer-cleanup 1s
    fi
  elif [ "${PACKAGE_TEST_TYPE:-other}" == "with-logstash" ] && [ "${package_to_test}" == "system_benchmark" ]; then
      echo "--- Run system benchmarks and report for package ${package_to_test}"
      elastic-package benchmark system -C "$d" --benchmark logs-benchmark -v --defer-cleanup 1s
  else
    if [[ "${SERVERLESS}" == "true" ]]; then
        echo "--- Run tests for package ${d} in Serverless mode"
        # skip system tests
        elastic-package test asset -C "$d" -v --report-format xUnit --report-output file --defer-cleanup 1s  --test-coverage --coverage-format=generic
        elastic-package test static -C "$d" -v --report-format xUnit --report-output file --defer-cleanup 1s  --test-coverage --coverage-format=generic
        # FIXME: adding test-coverage for serverless results in errors like this:
        # Error: error running package pipeline tests: could not complete test run: error calculating pipeline coverage: error fetching pipeline stats for code coverage calculations: need exactly one ES node in stats response (got 4)
        elastic-package test pipeline -C "$d" -v --report-format xUnit --report-output file --defer-cleanup 1s

        continue
    fi

    echo "--- Run tests for package ${d}"
    # Run all tests
    # defer-cleanup is set to a short period to verify that the option is available
    elastic-package test -C "$d" -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage --coverage-format=generic
  fi
done
