#!/bin/bash

# Tests the build and install flow for composable packages.
#
# The input package (01_ci_input_pkg) is built first so the local registry can
# serve it when the composable integration is built.  Both packages are then
# installed in Fleet to verify the full build → install path.
#
# Build order matters: the integration declares requires.input and its build
# step downloads ci_input_pkg from the local registry (stack.epr.base_url).
# The local registry is started by "elastic-package stack up" and serves
# packages from build/packages/.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_parameters.sh"
source "${SCRIPT_DIR}/stack_helpers.sh"

set -euxo pipefail

COMPOSABLE_PACKAGES_PATH="test/packages/composable"
COMPOSABLE_INPUT_PKG="01_ci_input_pkg"

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

echo "--- Create composable profile with local package registry URL"
elastic-package profiles create composable -v
elastic-package profiles use composable
mv ~/.elastic-package/profiles/composable/config.yml.example \
   ~/.elastic-package/profiles/composable/config.yml
# Point elastic-package commands to the local registry started by the stack.
echo "stack.epr.base_url: https://127.0.0.1:8080" \
  >> ~/.elastic-package/profiles/composable/config.yml

echo "--- Building input package: ${COMPOSABLE_PACKAGES_PATH}/${COMPOSABLE_INPUT_PKG}"
# The input package has no requires.input so no registry is needed at this stage.
# After build, the package lands in build/packages/ where the local registry serves it.
elastic-package build -C "${COMPOSABLE_PACKAGES_PATH}/${COMPOSABLE_INPUT_PKG}" -v

echo "--- Prepare Elastic stack"
stack_args=$(set +x; stack_version_args)
stack_args="${stack_args} $(set +x; stack_provider_args)"
elastic-package stack update -v ${stack_args}
# The local registry container serves packages from build/packages/, including the
# newly built input package.
elastic-package stack up -d -v ${stack_args}
elastic-package stack status

eval "$(elastic-package stack shellinit)"

echo "--- Installing input package: ${COMPOSABLE_PACKAGES_PATH}/${COMPOSABLE_INPUT_PKG}"
elastic-package install -C "${COMPOSABLE_PACKAGES_PATH}/${COMPOSABLE_INPUT_PKG}" -v

# Build and install each composable package that is not the input package.
# build downloads ci_input_pkg from the local registry and bundles it.
# install verifies the fully-composed package can be loaded by Fleet.
for d in "${COMPOSABLE_PACKAGES_PATH}"/*/; do
  package_to_test=$(basename "${d}")

  if [ "${package_to_test}" == "${COMPOSABLE_INPUT_PKG}" ]; then
    continue
  fi

  echo "--- Building package ${d}"
  elastic-package build -C "$d" -v

  echo "--- Installing package ${d}"
  elastic-package install -C "$d" -v
done
