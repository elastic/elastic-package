#!/bin/bash

# Orchestrates the with_script_upgrade script test, which exercises
# add_package_policy -version with a version that differs from the
# locally built package. Reproduces the regression in #3552.
#
# with_script_upgrade is a composable package that requires ci_input_pkg,
# exercising the composable code path in script tests (both with and without -version).
#
# The script:
# 1. Creates a dedicated profile so the default profile is not modified.
# 2. Builds ci_input_pkg before starting the registry so the local registry can
#    serve it when the integration package is built.
# 3. Boots only the package-registry service and points the profile at it so
#    elastic-package build can download ci_input_pkg while building with_script_upgrade.
# 4. Builds the original (previous) version of the package against the local registry.
# 5. Boots the full stack (the local registry now serves both the input package and
#    the previous package version).
# 6. Creates a dev copy of the package bumped to the next patch version.
# 7. Runs the upgrade script test against the dev copy (elastic-package test
#    builds the dev copy internally).

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source "${SCRIPT_DIR}/stack_helpers.sh"

set -euxo pipefail

INPUT_PKG_DIR="test/packages/composable/01_ci_input_pkg"
PACKAGE_DIR="test/packages/composable/08_with_script_upgrade"

WORK_DIR="build/dev-$(basename "${PACKAGE_DIR}")"
DEV_PACKAGE_DIR="${WORK_DIR}/$(basename "${PACKAGE_DIR}")"

cleanup() {
  local r=$?
  if [ "${r}" -ne 0 ]; then
    echo "^^^ +++"
  fi
  echo "~~~ elastic-package cleanup"

  if is_stack_created; then
    elastic-package stack dump -v \
      --output "build/elastic-stack-dump/script-upgrade" || true
    elastic-package stack down -v
  fi

  elastic-package profiles use default -v
  elastic-package profiles delete script-upgrade -v || true
  # The profile deletion above removes the config, but be explicit in case it fails.
  sed -i '/^stack\.epr\.base_url:/d' \
    ~/.elastic-package/profiles/script-upgrade/config.yml 2>/dev/null || true

  elastic-package clean -C "${INPUT_PKG_DIR}" -v
  elastic-package clean -C "${PACKAGE_DIR}" -v
  if [ -d "${DEV_PACKAGE_DIR}" ]; then
    elastic-package clean -C "${DEV_PACKAGE_DIR}" -v
  fi

  rm -rf "${WORK_DIR}"

  exit $r
}

trap cleanup EXIT

ELASTIC_PACKAGE_LINKS_FILE_PATH="$(pwd)/scripts/links_table.yml"
export ELASTIC_PACKAGE_LINKS_FILE_PATH

echo "--- Create profile"
elastic-package profiles delete script-upgrade -v || true
elastic-package profiles create script-upgrade -v
elastic-package profiles use script-upgrade
mv ~/.elastic-package/profiles/script-upgrade/config.yml.example \
   ~/.elastic-package/profiles/script-upgrade/config.yml

echo "--- Build input package (pre-registry, so local registry can serve it)"
elastic-package build -C "${INPUT_PKG_DIR}" -v

echo "--- Start package registry (needed to build the composable integration)"
elastic-package stack update -v
elastic-package stack up -d -v --services package-registry
elastic-package stack status

# Point the profile at the local registry only for building the composable
# package — this must NOT be set during stack up, as it would configure Kibana
# to use 127.0.0.1:8080 as EPR and break fleet-server enrollment.
echo "stack.epr.base_url: https://127.0.0.1:8080" \
  >> ~/.elastic-package/profiles/script-upgrade/config.yml

echo "--- Build previous (original) version for registry"
elastic-package build -C "${PACKAGE_DIR}" -v

# Remove the registry override before starting the full stack.
sed -i '/^stack\.epr\.base_url:/d' \
  ~/.elastic-package/profiles/script-upgrade/config.yml

echo "--- Start full stack"
elastic-package stack down -v
elastic-package stack up -d -v
elastic-package stack status

echo "--- Install input package"
elastic-package install -C "${INPUT_PKG_DIR}" -v

echo "--- Prepare dev copy with bumped version"
mkdir -p "${WORK_DIR}"
cp -r "${PACKAGE_DIR}" "${DEV_PACKAGE_DIR}"
elastic-package changelog add -C "${DEV_PACKAGE_DIR}" \
  --next patch \
  --description "Development version for upgrade test." \
  --type enhancement \
  --link "https://github.com/elastic/elastic-package/pull/1"

echo "--- Run upgrade script test"
# Add the override back so the composable build inside the test run works, and
# so install_package_from_registry can reach the local registry.
echo "stack.epr.base_url: https://127.0.0.1:8080" \
  >> ~/.elastic-package/profiles/script-upgrade/config.yml
elastic-package test -C "${DEV_PACKAGE_DIR}" -v \
  --report-format xUnit \
  --report-output file \
  --test-coverage \
  --coverage-format=generic

echo "--- Verify upgrade script test was executed"
if ! grep -r 'testcase name="script test: upgrade"' build/test-results/; then
  echo "ERROR: upgrade script test was not executed"
  exit 1
fi
