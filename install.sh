#!/usr/bin/env bash
set -euo pipefail

# claude-config installer
# Usage: curl -fsSL https://raw.githubusercontent.com/nizanrosh/claude-config-portable/main/install.sh | bash

REPO="nizanrosh/claude-config-portable"
BINARY="claude-config"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${CYAN}${BOLD}info${NC}  $*"; }
ok()    { echo -e "${GREEN}${BOLD}  ok${NC}  $*"; }
warn()  { echo -e "${YELLOW}${BOLD}warn${NC}  $*"; }
fail()  { echo -e "${RED}${BOLD}fail${NC}  $*"; exit 1; }

# --- Platform detection ---

detect_platform() {
    local os arch

    os="$(uname -s)"
    case "$os" in
        Darwin) os="darwin" ;;
        Linux)  os="linux"  ;;
        *)      fail "Unsupported OS: $os (only macOS and Linux are supported)" ;;
    esac

    arch="$(uname -m)"
    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        arm64)   arch="arm64" ;;
        *)       fail "Unsupported architecture: $arch" ;;
    esac

    PLATFORM="${os}"
    ARCH="${arch}"
}

# --- Check for gh CLI (needed for private repos) ---

USE_GH=false

check_gh() {
    if command -v gh &>/dev/null && gh auth status &>/dev/null 2>&1; then
        USE_GH=true
    fi
}

# --- Resolve latest version ---

resolve_version() {
    info "Resolving latest version..."

    if [ "$USE_GH" = true ]; then
        VERSION="$(gh release view --repo "${REPO}" --json tagName -q .tagName 2>/dev/null)" \
            || fail "Could not determine latest version. Check that the repo has releases."
    else
        VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')" \
            || fail "Could not determine latest version. For private repos, install gh CLI and run 'gh auth login'."
    fi

    if [ -z "$VERSION" ]; then
        fail "Could not determine latest version"
    fi
    ok "Latest version: ${VERSION}"
}

# --- Download & install ---

download_and_install() {
    local tarball="claude-config_${VERSION}_${PLATFORM}_${ARCH}.tar.gz"
    local tmpdir

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    info "Downloading ${tarball}..."
    if [ "$USE_GH" = true ]; then
        if ! gh release download "${VERSION}" --repo "${REPO}" --pattern "${tarball}" --dir "$tmpdir" 2>/dev/null; then
            fail "Download failed. Check that release ${VERSION} has artifact ${tarball}"
        fi
    else
        local url="https://github.com/${REPO}/releases/download/${VERSION}/${tarball}"
        if ! curl -fsSL "$url" -o "${tmpdir}/${tarball}"; then
            fail "Download failed. For private repos, install gh CLI and run 'gh auth login'."
        fi
    fi
    ok "Downloaded"

    info "Extracting..."
    tar -xzf "${tmpdir}/${tarball}" -C "$tmpdir"
    ok "Extracted"

    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        info "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi
    chmod +x "${INSTALL_DIR}/${BINARY}"
    ok "Installed to ${INSTALL_DIR}/${BINARY}"
}

# --- Verify ---

verify_install() {
    if ! command -v "$BINARY" &>/dev/null; then
        warn "${INSTALL_DIR} is not in your PATH"
        echo ""
        echo "  Add this to your shell profile (~/.zshrc or ~/.bashrc):"
        echo ""
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        return
    fi

    local installed_version
    installed_version="$("$BINARY" version 2>/dev/null || true)"
    ok "${installed_version}"
}

# --- Main ---

main() {
    echo ""
    echo -e "${BOLD}claude-config${NC} installer"
    echo ""

    detect_platform
    info "Platform: ${PLATFORM}/${ARCH}"

    check_gh
    resolve_version
    download_and_install
    verify_install

    echo ""
    echo -e "${GREEN}${BOLD}Done!${NC} Run ${CYAN}claude-config --help${NC} to get started."
    echo ""
}

main
