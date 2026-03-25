#!/usr/bin/env bash
# install.sh — One-line installer for DevRune CLI
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/davidarce/devrune/main/install.sh | bash
#
# Environment overrides:
#   DEVRUNE_VERSION   — pin a specific release tag (default: latest)
#   DEVRUNE_INSTALL   — override install directory (default: /usr/local/bin or ~/.local/bin)
#   GITHUB_REPO       — override repo slug (default: davidarce/devrune)

set -euo pipefail

REPO="${GITHUB_REPO:-davidarce/devrune}"
BINARY_NAME="devrune"

# ── Detect OS ──────────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
  Linux*)   OS_NAME="linux" ;;
  Darwin*)  OS_NAME="darwin" ;;
  *)
    echo "Error: Unsupported operating system: $OS" >&2
    echo "DevRune supports: linux, darwin (macOS)" >&2
    exit 1
    ;;
esac

# ── Detect Architecture ────────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64)  ARCH_NAME="amd64" ;;
  aarch64 | arm64) ARCH_NAME="arm64" ;;
  *)
    echo "Error: Unsupported architecture: $ARCH" >&2
    echo "DevRune supports: amd64, arm64" >&2
    exit 1
    ;;
esac

echo "Detected platform: ${OS_NAME}/${ARCH_NAME}"

# ── Resolve Version ────────────────────────────────────────────────────────────
if [ -n "${DEVRUNE_VERSION:-}" ]; then
  VERSION="$DEVRUNE_VERSION"
  echo "Using pinned version: $VERSION"
else
  echo "Fetching latest release from GitHub..."
  if ! VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'); then
    echo "Error: Failed to fetch latest release from GitHub." >&2
    echo "Check your internet connection or set DEVRUNE_VERSION to pin a version." >&2
    exit 1
  fi
  if [ -z "$VERSION" ]; then
    echo "Error: Could not determine latest release version." >&2
    exit 1
  fi
  echo "Latest version: $VERSION"
fi

# ── Build Download URL ─────────────────────────────────────────────────────────
# Expected asset name: devrune_<version>_<os>_<arch>.tar.gz
# Strip leading 'v' from version for asset name consistency.
VERSION_NUM="${VERSION#v}"
ASSET_NAME="${BINARY_NAME}_${VERSION_NUM}_${OS_NAME}_${ARCH_NAME}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"

echo "Downloading ${ASSET_NAME}..."

# ── Download Binary ────────────────────────────────────────────────────────────
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

TMP_ARCHIVE="${TMP_DIR}/${ASSET_NAME}"
if ! curl -fsSL --progress-bar --retry 3 --retry-delay 2 "$DOWNLOAD_URL" -o "$TMP_ARCHIVE"; then
  echo "" >&2
  echo "Error: Failed to download ${DOWNLOAD_URL}" >&2
  echo "Please check:" >&2
  echo "  1. The version '${VERSION}' exists: https://github.com/${REPO}/releases" >&2
  echo "  2. A binary exists for ${OS_NAME}/${ARCH_NAME}" >&2
  exit 1
fi

# ── Extract Binary ─────────────────────────────────────────────────────────────
echo "Extracting ${ASSET_NAME}..."
tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR"

TMP_BINARY="${TMP_DIR}/${BINARY_NAME}"
if [ ! -f "$TMP_BINARY" ]; then
  # Some releases may place the binary in a subdirectory.
  TMP_BINARY="$(find "$TMP_DIR" -type f -name "$BINARY_NAME" | head -n1)"
  if [ -z "$TMP_BINARY" ]; then
    echo "Error: Binary '${BINARY_NAME}' not found in archive." >&2
    exit 1
  fi
fi
chmod +x "$TMP_BINARY"

# ── Determine Install Directory ────────────────────────────────────────────────
if [ -n "${DEVRUNE_INSTALL:-}" ]; then
  INSTALL_DIR="$DEVRUNE_INSTALL"
elif [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  # Fallback to user-local bin (no root required).
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  # Warn if not on PATH.
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*)
      ;;
    *)
      echo "Warning: ${INSTALL_DIR} is not on your PATH." >&2
      echo "Add the following to your shell profile:" >&2
      echo "  export PATH=\"\$HOME/.local/bin:\$PATH\"" >&2
      ;;
  esac
fi

INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"
echo "Installing to ${INSTALL_PATH}..."

# ── Install ────────────────────────────────────────────────────────────────────
if [ -w "$INSTALL_DIR" ]; then
  cp "$TMP_BINARY" "$INSTALL_PATH"
else
  echo "Requesting sudo to install to ${INSTALL_DIR}..."
  sudo cp "$TMP_BINARY" "$INSTALL_PATH"
fi

# ── Verify Installation ────────────────────────────────────────────────────────
if ! command -v "$BINARY_NAME" &>/dev/null && [ ! -x "$INSTALL_PATH" ]; then
  echo "Error: Installation failed — binary not executable at ${INSTALL_PATH}" >&2
  exit 1
fi

echo ""
echo "DevRune ${VERSION} installed successfully!"
echo ""

# Print version to confirm.
if command -v "$BINARY_NAME" &>/dev/null; then
  "$BINARY_NAME" version 2>/dev/null || "$BINARY_NAME" --version 2>/dev/null || true
elif [ -x "$INSTALL_PATH" ]; then
  "$INSTALL_PATH" version 2>/dev/null || "$INSTALL_PATH" --version 2>/dev/null || true
fi

echo ""
echo "Get started:"
echo "  devrune --help"
echo "  devrune init"
