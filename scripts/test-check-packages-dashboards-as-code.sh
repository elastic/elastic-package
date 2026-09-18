#!/bin/bash

# Tests the dashboards-as-code build flow against a real Kibana.
#
# For each fixture under test/packages/dashboards_as_code/, this script:
#   1. Brings up the stack (the build step needs Kibana to compile dashboards).
#   2. Asserts the fixture has no compiled saved object yet (the build must
#      produce it, not just copy a pre-existing one).
#   3. Runs `elastic-package lint` then `elastic-package build
#      --compile-dashboards-as-code` (the equivalent of `check` but with
#      the opt-in flag wired to build).
#   4. Asserts that for every <name>.json source under
#      _dev/build/dashboards_as_code/ the build wrote
#      kibana/dashboard/<package>-<name>.json.
#
# The compiled dashboard files are not committed: any kibana/dashboard
# content that survives a run is removed in the cleanup trap.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_parameters.sh"
source "${SCRIPT_DIR}/stack_helpers.sh"

set -euxo pipefail

PACKAGES_PATH="test/packages/dashboards_as_code"

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if is_stack_created; then
    elastic-package stack dump -v \
      --output "build/elastic-stack-dump/check-dashboards-as-code" || true
    elastic-package stack down -v
  fi

  for d in "${PACKAGES_PATH}"/*/; do
    elastic-package clean -C "$d" -v || true
    # Remove freshly-compiled dashboard artifacts so the working tree stays clean.
    rm -rf "${d}kibana/dashboard"
    if [ -d "${d}kibana" ] && [ -z "$(ls -A "${d}kibana")" ]; then
      rmdir "${d}kibana"
    fi
  done

  exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

echo "--- Prepare Elastic stack"
stack_args=$(set +x; stack_version_args)
stack_args="${stack_args} $(set +x; stack_provider_args)"
elastic-package stack update -v ${stack_args}
elastic-package stack up -d -v ${stack_args}
elastic-package stack status

# Read package name from manifest.yml without a YAML parser.
package_name() {
  local manifest="$1/manifest.yml"
  awk -F': ' '$1 == "name" { gsub(/[" ]/, "", $2); print $2; exit }' "$manifest"
}

for d in "${PACKAGES_PATH}"/*/; do
  package_name=$(package_name "$d")
  echo "--- Checking package ${d} (name: ${package_name})"

  # Sanity-check the fixture: the compiled saved object must NOT exist yet.
  # If it does, the test would not actually be exercising the build step.
  if compgen -G "${d}kibana/dashboard/*.json" > /dev/null; then
    echo "Fixture ${d} already contains compiled dashboards; remove them so the test exercises the build step."
    exit 1
  fi

  elastic-package lint -C "$d" -v
  elastic-package build -C "$d" -v --compile-dashboards-as-code

  # For every source, the build must have written the standardised SO.
  for source in "${d}_dev/shared"/*.json; do
    source_id=$(basename "${source}" .json)
    expected="${d}kibana/dashboard/${package_name}-${source_id}.json"
    if [ ! -f "${expected}" ]; then
      echo "Build did not produce expected dashboard: ${expected}"
      exit 1
    fi
    echo "  OK: ${expected}"
  done
done
