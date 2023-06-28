#!/bin/bash

set -euxo pipefail


run_elastic_package_command() {
    if [ "x${CI_DEBUG_LOG_FILE_PATH:-}" != "x" ]; then
        local full_path="${OLDPWD}/${CI_DEBUG_LOG_FILE_PATH}"
        local folder=$(dirname ${full_path})

        elastic-package $@ 2>&1 /dev/stdout | tee ${full_path} | grep -v " DEBUG "
        exit 0
    fi
    elastic-package $@
}

cleanup() {
  r=$?

  # Dump stack logs
  run_elastic_package_command stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-any}}"

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
  run_elastic_package_command stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd $d
      run_elastic_package_command clean -v
    )
  done

  exit $r
}

trap cleanup EXIT

export ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"

OLDPWD=$PWD
# Build/check packages
for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd $d
    run_elastic_package_command check -v
  )
done
cd -

# Update the stack
run_elastic_package_command stack update -v

# Boot up the stack
run_elastic_package_command stack up -d -v

run_elastic_package_command stack status

if [ "${PACKAGE_TEST_TYPE:-other}" == "with-kind" ]; then
  # Boot up the kind cluster
  kind create cluster --config $PWD/scripts/kind-config.yaml
fi

# Run package tests
eval "$(run_elastic_package_command stack shellinit)"

for d in test/packages/${PACKAGE_TEST_TYPE:-other}/${PACKAGE_UNDER_TEST:-*}/; do
  (
    cd $d
    run_elastic_package_command install -v

    if [ "${PACKAGE_TEST_TYPE:-other}" == "benchmarks" ]; then
      # It is not used PACKAGE_UNDER_TEST, so all benchmark packages are run in the same loop
      package_to_test=$(basename ${d})
      if [ "${package_to_test}" == "pipeline_benchmark" ]; then
        rm -rf "${OLDPWD}/build/benchmark-results"
        run_elastic_package_command benchmark pipeline -v --report-format xUnit --report-output file --fail-on-missing

        rm -rf "${OLDPWD}/build/benchmark-results-old"
        mv "${OLDPWD}/build/benchmark-results" "${OLDPWD}/build/benchmark-results-old"

        run_elastic_package_command benchmark pipeline -v --report-format json --report-output file --fail-on-missing

        run_elastic_package_command report --fail-on-missing benchmark \
          --new ${OLDPWD}/build/benchmark-results \
          --old ${OLDPWD}/build/benchmark-results-old \
          --threshold 1 --report-output-path="${OLDPWD}/build/benchreport"
      fi
      # if [ "${package_to_test}" == "system_benchmark" ]; then
      #   run_elastic_package_command benchmark system --benchmark logs-benchmark -v
      # fi
    else
      # defer-cleanup is set to a short period to verify that the option is available
      run_elastic_package_command test -v --report-format xUnit --report-output file --defer-cleanup 1s --test-coverage
    fi
  )
cd -
done
