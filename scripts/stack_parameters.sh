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

default_stack_version() {
  grep "DefaultStackVersion =" internal/install/stack_version.go | awk '{print $3}' | tr -d '"'
}

# requires_newer_stack succeeds when the package pins, in its .stack_version
# file, a stack version newer than the given one, so it cannot be installed
# there.
requires_newer_stack() {
  local package_root="$1"
  local stack_version="${2%%-*}"
  local stack_version_file="${package_root%/}.stack_version"
  if [[ ! -f "$stack_version_file" ]]; then
    return 1
  fi

  local required
  required=$(cat "$stack_version_file")
  required="${required%%-*}"
  [[ "$required" != "$stack_version" && "$(printf '%s\n%s\n' "$required" "$stack_version" | sort -V | tail -n1)" == "$required" ]]
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
