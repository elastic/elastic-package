#!/bin/bash
set -euo pipefail

echo "Checking gsutil command..."
if ! command -v gsutil &> /dev/null ; then
    echo "⚠️  gsutil is not installed"
    exit 1
else
    echo "✅ gsutil is installed"
fi

gsutil help
