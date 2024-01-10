#!/bin/bash

set -euxo pipefail

function cleanup() {
  r=$?

  # Dump stack logs
  elastic-package stack dump -v --output "build/elastic-stack-dump/check-${PACKAGE_UNDER_TEST:-${PACKAGE_TEST_TYPE:-*}}"

  # Take down the stack
  elastic-package stack down -v

  # Clean used resources
  for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
    (
      cd "$d"
      elastic-package clean -v
    )
  done

  exit $r
}

function check_expected_errors() {
  local package_root=$1
  local package_name=""
  package_name=$(basename "$1")
  local expected_errors_file="${package_root%/}.expected_errors"
  local result_tests="build/test-results/${package_name}_*.xml"
  local results_no_spaces="build/test-results-no-spaces.xml"

  if [ ! -f "${expected_errors_file}" ]; then
    echo "No unexpected errors file in ${expected_errors_file}"
    return
  fi

  rm -f ${result_tests}
  (
    cd "$package_root"
    elastic-package test -v --report-format xUnit --report-output file --test-coverage --defer-cleanup 1s || true
  )

  cat ${result_tests} | tr -d '\n' > ${results_no_spaces}

  # check number of expected errors
  local number_errors
  number_errors=$(cat ${result_tests} | grep "<failure>" | wc -l)
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

  rm -f ${result_tests}
  rm -f ${results_no_spaces}
}

function check_build_output() {
  local package_root=$1
  local expected_build_output="${package_root%/}.build_output"
  local output_file="$PWD/build/elastic-package-output"

  if [ ! -f "${expected_build_output}" ]; then
    (
      cd "$package_root"
      elastic-package build -v
    )
    return
  fi

  (
    cd "$package_root"
    mkdir -p "$(dirname "$output_file")"
    elastic-package build 2>&1 | tee "$output_file" || true # Ignore errors here
  )

  diff -w -u "$expected_build_output" "$output_file" || (
    echo "Error: Build output has differences with expected output"
    exit 1
  )
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

# Update the stack
elastic-package stack update -v

# Boot up the stack
elastic-package stack up -d -v

elastic-package stack status

# Run package tests
for d in test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST:-*}/; do
  check_build_output "$d"
  check_expected_errors "$d"
done
