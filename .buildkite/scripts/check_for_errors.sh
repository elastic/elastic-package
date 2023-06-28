#!/bin/bash

set -euo pipefail


buildkite-agent artifact download "build/output-logs/*" .


for package_type in $(ls build/output-logs/); do
    for output_file in $(ls build/output-logs/${package_type}); do
        elastic-package-demo-output*.log ; do
        errors=$(grep -E "^Error:" $file)

        if [ -n "${errors}" ]; then
            cat <<EOF >> markdown.md
            - Error found in ${package_type}
              > ${errors}
EOF
        fi
done
done

if [ -f markdown.md ]; then
    cat markdown.md | buildkite-agent annotate --style error
fi
