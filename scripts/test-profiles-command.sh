#!/bin/bash

set -euxo pipefail

cleanup() {
  local r=$?

  # Delete extra profiles
  elastic-package profiles delete test_default

  exit $r
}

trap cleanup EXIT

# Force it to run and generate a default profile, if it hasn't already
elastic-package profiles list

# generate a new profile from default
elastic-package profiles create test_default

# generate from a non-default profile
elastic-package profiles create --from test_default test_from

# delete a profile
elastic-package profiles delete test_from