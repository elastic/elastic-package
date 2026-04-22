#!/bin/bash

# Tests the build and install flow for composable packages.
#
# Bootstrap input packages (01_ci_input_pkg, 05_ci_input_pkg_a, 06_ci_input_pkg_b)
# are built before the stack so the local registry can serve them when composable
# integrations are built (requires.input / dual-input fixtures). They are installed
# after stack up; the main loop skips them so they are not built or installed twice.
#
# Build order for bootstrap inputs is 01 → 05 → 06 before stack up. Integrations
# then build in directory order, downloading inputs from the registry (stack.epr.base_url).
#
# Stack version and provider settings reuse scripts/stack_parameters.sh:
# export PACKAGE_TEST_TYPE=composable and a non-empty PACKAGE_UNDER_TEST so
# stack_version_args / stack_provider_args read
# test/packages/composable/<PACKAGE_UNDER_TEST>.stack_version (and optional
# .stack_provider_settings). By default PACKAGE_UNDER_TEST is 01_ci_input_pkg;
# override with Makefile/CI PACKAGE_UNDER_TEST or COMPOSABLE_STACK_PIN_PACKAGE when
# PACKAGE_UNDER_TEST is empty.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# Prefer the elastic-package binary built in this repo (see Makefile `build`) so
# composable install uses the same code as the checkout, not an older global install.
if [[ -x "${REPO_ROOT}/elastic-package" ]]; then
  PATH="${REPO_ROOT}:${PATH}"
fi

source "${SCRIPT_DIR}/stack_parameters.sh"
source "${SCRIPT_DIR}/stack_helpers.sh"

set -euxo pipefail

COMPOSABLE_PACKAGES_PATH="test/packages/composable"
# Ordered bootstrap inputs: built pre-stack, installed post-stack, skipped in main loop.
COMPOSABLE_BOOTSTRAP_PKGS=("01_ci_input_pkg" "05_ci_input_pkg_a" "06_ci_input_pkg_b")
# Default stack pin (PACKAGE_UNDER_TEST) when unset; first bootstrap package.
COMPOSABLE_INPUT_PKG="${COMPOSABLE_BOOTSTRAP_PKGS[0]}"

is_composable_bootstrap_pkg() {
  local name="$1"
  for b in "${COMPOSABLE_BOOTSTRAP_PKGS[@]}"; do
    if [[ "${name}" == "${b}" ]]; then
      return 0
    fi
  done
  return 1
}

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if is_stack_created; then
    elastic-package stack dump -v \
      --output "build/elastic-stack-dump/composable" || true
    elastic-package stack down -v
  fi

  for d in "${COMPOSABLE_PACKAGES_PATH}"/*/; do
    elastic-package clean -C "$d" -v
  done

  elastic-package profiles delete composable -v || true
}

trap cleanup EXIT

echo "--- Create composable profile"
elastic-package profiles create composable -v
elastic-package profiles use composable
mv ~/.elastic-package/profiles/composable/config.yml.example \
   ~/.elastic-package/profiles/composable/config.yml

for bootstrap_name in "${COMPOSABLE_BOOTSTRAP_PKGS[@]}"; do
  echo "--- Building bootstrap input package: ${COMPOSABLE_PACKAGES_PATH}/${bootstrap_name}"
  # Bootstrap packages have no (or satisfied) requires.input; no registry yet.
  # After build, artifacts land in build/packages/ for the stack local registry.
  elastic-package build -C "${COMPOSABLE_PACKAGES_PATH}/${bootstrap_name}" -v
done

echo "--- Prepare Elastic stack"
export PACKAGE_TEST_TYPE=composable
if [[ -z "${PACKAGE_UNDER_TEST:-}" ]]; then
  export PACKAGE_UNDER_TEST="${COMPOSABLE_STACK_PIN_PACKAGE:-${COMPOSABLE_INPUT_PKG}}"
fi
stack_args=$(set +x; stack_version_args)
stack_args="${stack_args} $(set +x; stack_provider_args)"
elastic-package stack update -v ${stack_args}
# The local registry container serves packages from build/packages/, including the
# newly built input package.
elastic-package stack up -d -v ${stack_args}
elastic-package stack status

# Point elastic-package commands to the local registry started by the stack.
echo "stack.epr.base_url: https://127.0.0.1:8080" \
  >> ~/.elastic-package/profiles/composable/config.yml

# Kibana/registry clients use the active profile after stack up; shellinit is not required.

for bootstrap_name in "${COMPOSABLE_BOOTSTRAP_PKGS[@]}"; do
  echo "--- Installing bootstrap input package: ${COMPOSABLE_PACKAGES_PATH}/${bootstrap_name}"
  elastic-package install -C "${COMPOSABLE_PACKAGES_PATH}/${bootstrap_name}" -v
done

# Build and install each composable package that is not a bootstrap input.
# build may download required inputs from the local registry and bundle them.
# install verifies the fully-composed package can be loaded by Fleet.
for d in "${COMPOSABLE_PACKAGES_PATH}"/*/; do
  package_to_test=$(basename "${d}")

  if is_composable_bootstrap_pkg "${package_to_test}"; then
    continue
  fi

  echo "--- Building package ${d}"
  elastic-package build -C "$d" -v

  echo "--- Installing package ${d}"
  elastic-package install -C "$d" -v
done
