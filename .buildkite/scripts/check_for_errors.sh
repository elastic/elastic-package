#!/bin/bash

set -euo pipefail


mkdir -p build
buildkite-agent artifact download "build/output-logs/*" build/

echo ----
ls -lR build
echo ----

for package_type in $(ls build/output-logs/); do
    for output_file in $(ls build/output-logs/${package_type}); do
        output="build/output-logs/${package_type}/${output_file}"
        echo "Any error on ${output}?"
        errors=$(grep -E "^Error:" ${output})

        echo "Found errors":
        echo "${errors}"

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
