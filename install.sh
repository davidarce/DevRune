#!/usr/bin/env bash
# install.sh — One-line installer for DevRune CLI
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | bash
#
# Pin a specific version:
#   curl -fsSL https://raw.githubusercontent.com/davidarce/DevRune/main/install.sh | VERSION=v0.1.0 bash
#
# Override install directory:
#   curl -fsSL ... | INSTALL_DIR=/usr/local/bin bash
#
# Environment variables:
#   VERSION       — release tag to install (default: latest)
#   INSTALL_DIR   — where to place the binary (default: ~/.local/bin)

set -euo pipefail

REPO="davidarce/DevRune"
BINARY_NAME="devrune"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${VERSION:-latest}"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf "${CYAN}▸${NC} %s\n" "$1"; }
ok()    { printf "${GREEN}✓${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}!${NC} %s\n" "$1"; }
error() { printf "${RED}✗${NC} %s\n" "$1" >&2; exit 1; }

# --- Detect OS and Architecture ---
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        darwin) os="darwin" ;;
        linux)  os="linux" ;;
        mingw*|msys*|cygwin*) error "Windows is not supported. Use WSL instead." ;;
        *) error "Unsupported operating system: $os" ;;
    esac

    case "$arch" in
        x86_64|amd64)   arch="amd64" ;;
        arm64|aarch64)  arch="arm64" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac

    if [[ "$os" == "linux" && "$arch" == "arm64" ]]; then
        error "linux/arm64 is not available. Supported: darwin-arm64, darwin-amd64, linux-amd64"
    fi

    PLATFORM_OS="$os"
    PLATFORM_ARCH="$arch"
}

# --- Resolve version tag ---
resolve_version() {
    if [[ "$VERSION" == "latest" ]]; then
        info "Resolving latest release..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' \
            | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)

        if [[ -z "$VERSION" || "$VERSION" == "null" ]]; then
            error "Could not resolve latest version. Check https://github.com/${REPO}/releases"
        fi
    fi
    ok "Version: ${VERSION}"
}

# --- Download and extract ---
download_binary() {
    local version_num="${VERSION#v}"
    local asset_name="${BINARY_NAME}_${version_num}_${PLATFORM_OS}_${PLATFORM_ARCH}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${VERSION}/${asset_name}"

    TMP_DIR="$(mktemp -d)"
    trap 'rm -rf "$TMP_DIR"' EXIT

    info "Downloading ${asset_name}..."
    if ! curl -fsSL --progress-bar --retry 3 --retry-delay 2 "$download_url" -o "${TMP_DIR}/${asset_name}"; then
        error "Download failed. Check that ${VERSION} exists: https://github.com/${REPO}/releases"
    fi

    info "Extracting..."
    tar -xzf "${TMP_DIR}/${asset_name}" -C "$TMP_DIR"

    DOWNLOADED_FILE="${TMP_DIR}/${BINARY_NAME}"
    if [[ ! -f "$DOWNLOADED_FILE" ]]; then
        DOWNLOADED_FILE="$(find "$TMP_DIR" -type f -name "$BINARY_NAME" | head -n1)"
        if [[ -z "$DOWNLOADED_FILE" ]]; then
            error "Binary '${BINARY_NAME}' not found in archive."
        fi
    fi
    chmod +x "$DOWNLOADED_FILE"
    ok "Downloaded ${BINARY_NAME} ${VERSION}"
}

# --- Install binary ---
install_binary() {
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mkdir -p "$INSTALL_DIR"
    mv "$DOWNLOADED_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
    ok "Installed: ${INSTALL_DIR}/${BINARY_NAME}"
}

# --- Verify PATH ---
check_path() {
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        echo ""
        warn "${INSTALL_DIR} is not in your PATH."
        echo ""
        printf "  Add it to your shell profile:\n"

        local shell_name
        shell_name="$(basename "${SHELL:-/bin/bash}")"
        local profile_file
        case "$shell_name" in
            zsh)  profile_file="~/.zshrc" ;;
            bash) profile_file="~/.bashrc" ;;
            fish) profile_file="~/.config/fish/config.fish" ;;
            *)    profile_file="~/.profile" ;;
        esac

        if [[ "$shell_name" == "fish" ]]; then
            printf "    ${DIM}echo 'set -gx PATH %s \$PATH' >> %s${NC}\n" "$INSTALL_DIR" "$profile_file"
        else
            printf "    ${DIM}echo 'export PATH=\"%s:\$PATH\"' >> %s${NC}\n" "$INSTALL_DIR" "$profile_file"
        fi
        echo ""
    fi
}

# --- Verify installation ---
verify() {
    local installed="${INSTALL_DIR}/${BINARY_NAME}"

    if [[ ! -x "$installed" ]]; then
        error "Installation failed: ${installed} is not executable"
    fi

    local version_output
    version_output=$("$installed" version 2>&1 || "$installed" --version 2>&1 || true)

    echo ""
    printf "${BOLD}${GREEN}"
    echo "  ╔══════════════════════════════════════╗"
    echo "  ║        DevRune installed!            ║"
    echo "  ╚══════════════════════════════════════╝"
    printf "${NC}\n"
    printf "  ${DIM}%s${NC}\n" "$version_output"
    echo ""
    printf "  Run ${CYAN}devrune init${NC} in any project to get started.\n"
    printf "  Run ${CYAN}devrune --help${NC} for all options.\n"
    echo ""
}

# --- Main ---
main() {
    echo ""
    printf "${BOLD}DevRune installer${NC}\n"
    echo ""

    detect_platform
    ok "Platform: ${PLATFORM_OS}/${PLATFORM_ARCH}"

    resolve_version
    download_binary
    install_binary
    check_path
    verify
}

main
