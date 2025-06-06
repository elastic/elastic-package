#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_parameters.sh"

set -euxo pipefail

function cleanup() {
  r=$?
  if [ "${r}" -ne 0 ]; then
    # Ensure that the group where the failure happened is opened.
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if [ "${ELASTIC_PACKAGE_STARTED}" -eq 1 ]; then
    # Dump stack logs
    elastic-package stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-*}}"
  fi

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
    elastic-package clean -C "$d" -v
  done

  exit $r
}

function check_expected_errors() {
  local package_root=$1
  local package_name=""
  local package_name_manifest=""
  package_name=$(basename "$1")
  package_name_manifest=$(cat "$package_root/manifest.yml" | yq -r '.name')
  local expected_errors_file="${package_root%/}.expected_errors"
  local result_tests="build/test-results/${package_name_manifest}-*.xml"
  local results_no_spaces="build/test-results-no-spaces.xml"

  if [ ! -f "${expected_errors_file}" ]; then
    echo "No unexpected errors file in ${expected_errors_file}"
    return
  fi

  rm -f ${result_tests}
  elastic-package test -C "$package_root" -v --report-format xUnit --report-output file --test-coverage --coverage-format=generic --defer-cleanup 1s || true

  cat ${result_tests} | tr -d '\n' > ${results_no_spaces}

  # check number of expected errors
  local number_errors
  number_errors=$(cat ${result_tests} | grep -E '<failure>|<error>' | wc -l)
  local expected_errors
  expected_errors=$(cat ${expected_errors_file} | wc -l)

  if [ "${number_errors}" -ne "${expected_errors}" ]; then
      echo "Error: There are unexpected errors in ${package_name}"
      exit 1
  fi

  # check whether or not the expected errors exist in the xml files
  while read -r line; do
    cat ${results_no_spaces} | grep -E "${line}"
  done < "${expected_errors_file}"

  # Copy XML files to another extension so they are not used to check jUnit tests
  # but those files will be able to be reviewed afterwards
  for file in $(ls $result_tests) ; do
      cp "${file}" "${file}.expected-errors.txt"
  done
  rm -f ${result_tests}
  rm -f ${results_no_spaces}
}

function check_build_output() {
  local package_root=$1
  local expected_build_output="${package_root%/}.build_output"
  local output_file="$PWD/build/elastic-package-output"

  if [ ! -f "${expected_build_output}" ]; then
    elastic-package build -C "$package_root" -v
    return
  fi

  mkdir -p "$(dirname "$output_file")"
  elastic-package build -C "$package_root" 2>&1 | tee "$output_file" || true # Ignore errors here

  diff -w -u "$expected_build_output" "$output_file" || (
    echo "Error: Build output has differences with expected output"
    exit 1
  )
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH
ELASTIC_PACKAGE_STARTED=0

stack_args=$(stack_version_args) # --version <version>

echo "--- Prepare Elastic stack"
# Update the stack
elastic-package stack update -v ${stack_args}

# NOTE: if any provider argument is defined, the stack must be shutdown first to ensure
# that all parameters are taken into account by the services
stack_args="${stack_args} $(stack_provider_args)" # -U <setting=1,settings=2>

# Boot up the stack
elastic-package stack up -d -v ${stack_args}

ELASTIC_PACKAGE_STARTED=1
elastic-package stack status

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
  echo "--- Check build output: ${d}"
  check_build_output "$d"
  echo "--- Check expected errors: ${d}"
  check_expected_errors "$d"
done
