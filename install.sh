#!/bin/bash
set -e

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
REPO="inference-gateway/inference-gateway"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

error() {
    echo -e "${RED}Error:${NC} $1" >&2
    exit 1
}

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "Linux";;
        Darwin*)    echo "Darwin";;
        *)          error "Unsupported operating system: $(uname -s)";;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "x86_64";;
        aarch64|arm64)  echo "arm64";;
        armv7l)         echo "armv7";;
        *)              error "Unsupported architecture: $(uname -m)";;
    esac
}

get_download_url() {
    local os="$1"
    local arch="$2"
    local version="$3"

    if [ "$version" = "latest" ]; then
        local release_url="https://api.github.com/repos/${REPO}/releases/latest"
    else
        local release_url="https://api.github.com/repos/${REPO}/releases/tags/${version}"
    fi

    local release_json=$(curl -fsSL "$release_url")

    local tag_name=$(echo "$release_json" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/"tag_name": *"\(.*\)"/\1/')

    local asset_name="inference-gateway_${os}_${arch}.tar.gz"

    local download_url=$(echo "$release_json" | grep -o "\"browser_download_url\": *\"[^\"]*${asset_name}\"" | sed 's/"browser_download_url": *"\(.*\)"/\1/')

    if [ -z "$download_url" ]; then
        error "Could not find binary for ${os}/${arch} in release ${tag_name}"
    fi

    echo "$download_url"
}

main() {
    info "Installing Inference Gateway..."

    local os=$(detect_os)
    local arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    info "Fetching release information..."
    local download_url=$(get_download_url "$os" "$arch" "$VERSION")

    info "Downloading from: $download_url"

    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    local archive_path="${tmp_dir}/inference-gateway.tar.gz"
    if ! curl -fsSL -o "$archive_path" "$download_url"; then
        error "Failed to download binary"
    fi

    info "Extracting binary..."
    tar -xzf "$archive_path" -C "$tmp_dir"

    local binary_path=$(find "$tmp_dir" -name "inference-gateway" -type f | head -1)

    if [ -z "$binary_path" ]; then
        error "Binary not found in archive"
    fi

    chmod +x "$binary_path"

    info "Installing to ${INSTALL_DIR}..."

    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary_path" "${INSTALL_DIR}/inference-gateway"
    else
        warn "Need sudo permissions to install to ${INSTALL_DIR}"
        sudo mv "$binary_path" "${INSTALL_DIR}/inference-gateway"
    fi

    if command -v inference-gateway &> /dev/null; then
        local installed_version=$(inference-gateway --version 2>/dev/null || echo "unknown")
        info "✓ Successfully installed inference-gateway"
        info "Version: $installed_version"
        info "Location: ${INSTALL_DIR}/inference-gateway"
    else
        error "Installation failed - binary not found in PATH"
    fi
}

usage() {
    cat <<EOF
Usage: install.sh [OPTIONS]

Install the inference-gateway binary from GitHub releases.

Options:
    VERSION=<version>       Specify version to install (default: latest)
    INSTALL_DIR=<path>      Installation directory (default: /usr/local/bin)

Examples:
    # Install latest version to /usr/local/bin
    curl -fsSL https://raw.githubusercontent.com/inference-gateway/inference-gateway/main/install.sh | bash

    # Install specific version
    curl -fsSL https://raw.githubusercontent.com/inference-gateway/inference-gateway/main/install.sh | VERSION=v1.2.3 bash

    # Install to custom directory
    curl -fsSL https://raw.githubusercontent.com/inference-gateway/inference-gateway/main/install.sh | INSTALL_DIR=~/.local/bin bash

    # Install to current directory
    curl -fsSL https://raw.githubusercontent.com/inference-gateway/inference-gateway/main/install.sh | INSTALL_DIR=. bash

EOF
}

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
    exit 0
fi

main
