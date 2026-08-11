#!/bin/bash
# Install a release of elastic-package for the current platform.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/install.sh | bash -s -- --version 0.125.0
#   INSTALL_DIR=$HOME/.local/bin bash install.sh

set -euo pipefail

REPO="elastic/elastic-package"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Parse arguments
REQUESTED_VERSION=""
while [ $# -gt 0 ]; do
  case "$1" in
    --version|-v)
      REQUESTED_VERSION="${2:-}"
      if [ -z "$REQUESTED_VERSION" ]; then
        echo "Error: --version requires a value." >&2
        exit 1
      fi
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: install.sh [--version <version>]" >&2
      exit 1
      ;;
  esac
done

# Strip leading 'v' from version if provided (accept both v0.125.0 and 0.125.0)
REQUESTED_VERSION="${REQUESTED_VERSION#v}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  MINGW*|MSYS*|CYGWIN*)
    echo "Windows is not supported by this script." >&2
    echo "Use the PowerShell script instead:" >&2
    echo "  irm https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/install.ps1 | iex" >&2
    exit 1
    ;;
  *)
    echo "Unsupported operating system: $OS" >&2
    echo "Please download the binary manually from https://github.com/${REPO}/releases/latest" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  i386|i686)     ARCH="386" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    echo "Please download the binary manually from https://github.com/${REPO}/releases/latest" >&2
    exit 1
    ;;
esac

# Resolve target version: use requested version or follow the GitHub
# /releases/latest redirect (avoids JSON parsing and API rate limits).
if [ -n "$REQUESTED_VERSION" ]; then
  VERSION="$REQUESTED_VERSION"
else
  echo "Fetching latest elastic-package release..."
  LATEST_URL=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest")
  VERSION="${LATEST_URL##*/}"
  VERSION="${VERSION#v}"

  if [ -z "$VERSION" ]; then
    echo "Failed to determine the latest release version." >&2
    exit 1
  fi
fi

# Check currently installed version, skip if already at the target version.
# Prefer the binary in INSTALL_DIR (matches install.ps1), then fall back to PATH.
CURRENT_VERSION=""
EXISTING_BINARY="${INSTALL_DIR}/elastic-package"
if [ -x "$EXISTING_BINARY" ]; then
  CURRENT_VERSION=$("$EXISTING_BINARY" version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
elif command -v elastic-package >/dev/null 2>&1; then
  CURRENT_VERSION=$(elastic-package version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
fi

if [ -n "$CURRENT_VERSION" ] && [ "$CURRENT_VERSION" = "$VERSION" ]; then
  echo "elastic-package v${VERSION} is already installed and up to date."
  exit 0
fi

if [ -n "$CURRENT_VERSION" ]; then
  echo "Updating elastic-package v${CURRENT_VERSION} -> v${VERSION} (${OS}/${ARCH})..."
else
  echo "Installing elastic-package v${VERSION} (${OS}/${ARCH})..."
fi

FILENAME="elastic-package_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/elastic-package_${VERSION}_checksums.txt"

# mktemp -d creates a 0700 directory — only the current user can read it
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# Download archive to disk (needed for checksum verification before extraction)
ARCHIVE="${TMP_DIR}/${FILENAME}"
curl -fsSL "$URL" -o "$ARCHIVE"

# Verify checksum against the release checksums file
CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"
curl -fsSL "$CHECKSUMS_URL" -o "$CHECKSUMS_FILE"

EXPECTED_HASH=$(grep "  ${FILENAME}$" "$CHECKSUMS_FILE" | awk '{print $1}')
if [ -z "$EXPECTED_HASH" ]; then
  echo "Checksum not found for ${FILENAME} in checksums file." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  echo "${EXPECTED_HASH}  ${ARCHIVE}" | sha256sum -c - >/dev/null
  echo "Checksum verified."
elif command -v shasum >/dev/null 2>&1; then
  echo "${EXPECTED_HASH}  ${ARCHIVE}" | shasum -a 256 -c - >/dev/null
  echo "Checksum verified."
else
  echo "Warning: cannot verify checksum (neither sha256sum nor shasum found)." >&2
fi

# Extract archive
tar -xz -C "$TMP_DIR" -f "$ARCHIVE"

BINARY="${TMP_DIR}/elastic-package"

if [ ! -f "$BINARY" ]; then
  echo "Extraction failed: elastic-package binary not found in archive." >&2
  exit 1
fi

chmod +x "$BINARY"

# Remove quarantine attribute on macOS to allow the binary to run
if [ "$OS" = "darwin" ]; then
  xattr -r -d com.apple.quarantine "$BINARY" 2>/dev/null || true
fi

# Ensure the install directory exists; fall back to sudo if the parent is root-owned
if [ ! -d "$INSTALL_DIR" ]; then
  echo "Creating install directory ${INSTALL_DIR}..."
  mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi

# Install the binary
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "${INSTALL_DIR}/elastic-package"
else
  echo "Installing to ${INSTALL_DIR} requires elevated privileges..."
  sudo mv "$BINARY" "${INSTALL_DIR}/elastic-package"
fi

echo ""
echo "elastic-package v${VERSION} installed to ${INSTALL_DIR}/elastic-package"
echo ""

# Verify installation
if command -v elastic-package >/dev/null 2>&1; then
  elastic-package version
else
  echo "Make sure ${INSTALL_DIR} is in your PATH."
fi
