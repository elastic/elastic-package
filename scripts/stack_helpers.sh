#!/usr/bin/env bash

set -euo pipefail

is_stack_created() {
    local files=0
    files=$(find ~/.elastic-package -type f -name "docker-compose.yml" | wc -l)
    if [ "${files}" -gt 0 ]; then
        return 0
    fi
    return 1
}

