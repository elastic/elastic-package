#!/usr/bin/env bash

stack_version_args() {
  if [[ -z "${PACKAGE_UNDER_TEST:-""}" ]]; then
    # Don't force stack version if we are testing multiple packages.
    return
  fi

  local package_root=test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST}/
  local stack_version_file="${package_root%/}.stack_version"
  if [[ ! -f "$stack_version_file" ]]; then
    return
  fi

  echo -n "--version $(cat "$stack_version_file")"
}

stack_provider_args() {
  if [[ -z "${PACKAGE_UNDER_TEST:-""}" ]]; then
    # Don't force stack version if we are testing multiple packages.
    return
  fi

  local package_root=test/packages/${PACKAGE_TEST_TYPE:-false_positives}/${PACKAGE_UNDER_TEST}/
  local parameters="${package_root%/}.stack_provider_settings"
  if [[ ! -f "$parameters" ]]; then
    return
  fi

  echo -n "-U $(cat "${parameters}" | tr '\n' ',' | tr -d ' ' | sed 's/,$//')"
}
