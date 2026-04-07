#!/bin/bash
# Demo script for elastic-package LSP
# Usage: ./scripts/lsp-demo.sh [path-to-binary]
#
# Runs 3 scenarios:
#   1. Valid package (apache) — expect no errors
#   2. Broken package — expect multiple errors routed to correct files
#   3. Open a nested data_stream file — LSP discovers package root

set -euo pipefail

BINARY="${1:-./elastic-package}"

if [[ ! -x "$BINARY" ]]; then
  echo "Building elastic-package..."
  go build -o "$BINARY" .
fi

send_msg() {
  local msg="$1"
  local len=${#msg}
  printf "Content-Length: %d\r\n\r\n%s" "$len" "$msg"
}

run_lsp() {
  local uri="$1"
  local init='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}'
  local initialized='{"jsonrpc":"2.0","method":"initialized","params":{}}'
  local didopen="{\"jsonrpc\":\"2.0\",\"method\":\"textDocument/didOpen\",\"params\":{\"textDocument\":{\"uri\":\"$uri\",\"languageId\":\"yaml\",\"version\":1,\"text\":\"\"}}}"
  local shutdown='{"jsonrpc":"2.0","id":2,"method":"shutdown"}'
  local exit_msg='{"jsonrpc":"2.0","method":"exit"}'

  {
    send_msg "$init"; sleep 0.3
    send_msg "$initialized"; sleep 0.3
    send_msg "$didopen"; sleep 3
    send_msg "$shutdown"; sleep 0.3
    send_msg "$exit_msg"
  } | "$BINARY" lsp 2>/dev/null
}

print_diags() {
  python3 -c "
import sys, json

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    msg = json.loads(line)
    uri = msg['params']['uri']
    diags = msg['params']['diagnostics']
    parts = uri.replace('file://', '').split('/')
    # Show last 3-4 path components
    short = '/'.join(parts[-4:]) if len(parts) > 4 else '/'.join(parts)
    print(f'  File: {short}')
    if diags:
        for d in diags:
            code = d.get('code', '')
            code_str = f' ({code})' if code else ''
            sev = {1: 'ERROR', 2: 'WARN', 3: 'INFO', 4: 'HINT'}.get(d.get('severity', 1), '?')
            print(f'    [{sev}] {d[\"message\"]}{code_str}')
    else:
        print('    (no errors)')
    print()
"
}

extract_diags() {
  sed 's/Content-Length: [0-9]*/\
---/g' | grep publishDiagnostics | print_diags
}

# ─── Scenario 1: Valid package ────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INTEGRATIONS_DIR="$(cd "$REPO_ROOT/../integrations/packages" 2>/dev/null && pwd)" || true

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Scenario 1: Valid package (apache)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [[ -n "$INTEGRATIONS_DIR" && -f "$INTEGRATIONS_DIR/apache/manifest.yml" ]]; then
  run_lsp "file://$INTEGRATIONS_DIR/apache/manifest.yml" | extract_diags
else
  echo "  (skipped — ../integrations/packages/apache not found)"
  echo ""
fi

# ─── Scenario 2: Broken package ──────────────────────────────────────────────

BROKEN_DIR=$(mktemp -d)
trap 'rm -rf "$BROKEN_DIR"' EXIT

mkdir -p "$BROKEN_DIR/data_stream/mystream"
cat > "$BROKEN_DIR/manifest.yml" << 'YAML'
format_version: 3.3.0
name: INVALID-NAME
title: ""
version: 0.0.1
type: integration
description: A broken package for testing
categories:
  - security
conditions:
  kibana:
    version: "^8.0.0"
owner:
  github: elastic/test
YAML
cat > "$BROKEN_DIR/data_stream/mystream/manifest.yml" << 'YAML'
title: My Stream
type: logs
YAML

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Scenario 2: Broken package (invalid name, missing files)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
run_lsp "file://$BROKEN_DIR/manifest.yml" | extract_diags

# ─── Scenario 3: Open nested file ────────────────────────────────────────────

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Scenario 3: Open data_stream file — LSP discovers package root"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
run_lsp "file://$BROKEN_DIR/data_stream/mystream/manifest.yml" | extract_diags

echo "Done."
