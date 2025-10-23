#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_parameters.sh"
source "${SCRIPT_DIR}/stack_helpers.sh"

set -euxo pipefail

# Add default values
export SUFFIX_FOLDER_DUMP_LOGS="${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-any}}"
export PACKAGE_TEST_TYPE="${PACKAGE_TEST_TYPE:-"other"}"
export PACKAGE_UNDER_TEST="${PACKAGE_UNDER_TEST:-*}"

cleanup() {
  r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if is_stack_created; then
    # Dump stack logs
    # Required containers could not be running, so ignore the error
    elastic-package stack dump -v \
      --output "build/elastic-stack-dump/check-${SUFFIX_FOLDER_DUMP_LOGS}" || true
  fi

  if [ "${PACKAGE_TEST_TYPE}" == "with-kind" ]; then
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
  if [[ "$SERVERLESS" != "true" ]]; then
    if is_stack_created; then
      # Take down the stack
      elastic-package stack down -v
    fi
  fi

  if [ "${PACKAGE_TEST_TYPE}" == "with-logstash" ]; then
    # Delete the logstash profile
    elastic-package profiles delete logstash -v
  fi

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE}/${PACKAGE_UNDER_TEST}/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH
export SERVERLESS=${SERVERLESS:-"false"}

run_system_benchmark() {
  local package_name="$1"
  local package_path="$2"

  local benchmark_file_path=""
  local benchmark_filename=""
  local benchmark_name=""

  for benchmark_file_path in $(find "${package_path}/_dev/benchmark/system/" -maxdepth 1 -mindepth 1 -type f -name "*.yml" ) ; do
      benchmark_filename="$(basename "${benchmark_file_path}")"
      benchmark_name="${benchmark_filename%.*}"
      echo "--- Run system benchmarks for package ${package_name} - ${benchmark_name}"
      elastic-package benchmark system -C "$package_path" --benchmark "${benchmark_name}" -v --defer-cleanup 1s
  done
}

run_serverless_tests() {
  local package_path="$1"
  local test_options="-v --report-format xUnit --report-output file --defer-cleanup 1s"
  local coverage_options="--test-coverage --coverage-format=generic"

  echo "--- Run tests for package ${package_path} in Serverless mode"
  # skip system tests
  elastic-package test asset -C "$package_path" $test_options $coverage_options
  elastic-package test static -C "$package_path" $test_options $coverage_options
  # FIXME: adding test-coverage for serverless results in errors like this:
  # Error: error running package pipeline tests: could not complete test run: error calculating pipeline coverage: error fetching pipeline stats for code coverage calculations: need exactly one ES node in stats response (got 4)
  elastic-package test pipeline -C "$package_path" $test_options
}

run_pipeline_benchmark() {
  local package_name="$1"
  local package_path="$2"
  local test_options="-v --report-format xUnit --report-output file --fail-on-missing"

  echo "--- Run pipeline benchmarks and report for package ${package_name}"

  rm -rf "${PWD}/build/benchmark-results"
  elastic-package benchmark pipeline -C "$d" $test_options

  rm -rf "${PWD}/build/benchmark-results-old"
  mv "${PWD}/build/benchmark-results" "${PWD}/build/benchmark-results-old"

  elastic-package benchmark pipeline -C "$d" $test_options

  elastic-package report -C "$d" --fail-on-missing benchmark \
    --new "${PWD}/build/benchmark-results" \
    --old "${PWD}/build/benchmark-results-old" \
    --threshold 1 --report-output-path="${PWD}/build/benchreport"
}


# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE}/${PACKAGE_UNDER_TEST}/; do
  echo "--- Checking package ${d}"
  elastic-package check -C "$d" -v
done

if [ "${PACKAGE_TEST_TYPE}" == "with-logstash" ]; then
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
if [[ "${SERVERLESS}" != "true" ]]; then
  echo "--- Prepare Elastic stack"
  stack_args=$(set +x;stack_version_args) # --version <version>

  # Update the stack
  elastic-package stack update -v ${stack_args}

  # NOTE: if any provider argument is defined, the stack must be shutdown first to ensure
  # that all parameters are taken into account by the services
  stack_args="${stack_args} $(set +x; stack_provider_args)" # -U <setting=1,settings=2>

  # Boot up the stack
  elastic-package stack up -d -v ${stack_args}

  elastic-package stack status
fi

if [ "${PACKAGE_TEST_TYPE}" == "with-kind" ]; then
  # Boot up the kind cluster
  echo "--- Create kind cluster"
  kind create cluster --config "$PWD/scripts/kind-config.yaml" --image "kindest/node:${K8S_VERSION}"
fi

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE}/${PACKAGE_UNDER_TEST}/; do
  package_to_test=$(basename "${d}")

  if [ "${PACKAGE_TEST_TYPE}" == "benchmarks" ]; then
    # FIXME: There are other packages in test/packages/benchmarks folder that are not tested like rally_benchmark
    case "${package_to_test}" in
      pipeline_benchmark|use_pipeline_tests)
        run_pipeline_benchmark "${package_to_test}" "$d"
        ;;
      system_benchmark)
        run_system_benchmark "${package_to_test}" "$d"
        ;;
    esac
    continue
  fi

  if [ "${PACKAGE_TEST_TYPE}" == "with-logstash" ] && [ "${package_to_test}" == "system_benchmark" ]; then
    run_system_benchmark "${package_to_test}" "$d"
    continue
  fi

  if [[ "${SERVERLESS}" == "true" ]]; then
    run_serverless_tests "${d}"
    continue
  fi

  echo "--- Run tests for package ${d}"
  # Run all tests
  # defer-cleanup is set to a short period to verify that the option is available
  elastic-package test -C "$d" -v \
    --report-format xUnit \
    --report-output file \
    --defer-cleanup 1s \
    --test-coverage \
    --coverage-format=generic
done
