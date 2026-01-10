#!/bin/bash

set -e

# dbtree installer script
# Usage: curl -fsSL https://vivekn.dev/dbtree/install.sh | bash

REPO="viveknathani/dbtree"
BINARY_NAME="dbtree"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    local os
    case "$(uname -s)" in
        Linux*)     os="linux";;
        Darwin*)    os="darwin";;
        MINGW*|MSYS*|CYGWIN*) os="windows";;
        *)          error "Unsupported operating system: $(uname -s)";;
    esac
    echo "$os"
}

# Detect architecture
detect_arch() {
    local arch
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64";;
        aarch64|arm64)  arch="arm64";;
        *)              error "Unsupported architecture: $(uname -m)";;
    esac
    echo "$arch"
}

# Get the latest release version from GitHub
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to get latest version"
    fi
    echo "$version"
}

# Download and install
install_dbtree() {
    local os=$(detect_os)
    local arch=$(detect_arch)
    local version=$(get_latest_version)

    info "Detected OS: $os"
    info "Detected architecture: $arch"
    info "Latest version: $version"

    # Construct download URL
    local file_ext="tar.gz"
    if [ "$os" = "windows" ]; then
        file_ext="zip"
    fi

    local archive_name="${BINARY_NAME}_${version#v}_${os}_${arch}.${file_ext}"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

    info "Downloading from: $download_url"

    # Create temporary directory
    local tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    # Download
    if ! curl -fsSL -o "$archive_name" "$download_url"; then
        error "Failed to download $archive_name"
    fi

    # Extract
    info "Extracting..."
    if [ "$file_ext" = "tar.gz" ]; then
        tar -xzf "$archive_name"
    else
        unzip -q "$archive_name"
    fi

    # Install
    info "Installing to $INSTALL_DIR..."

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_NAME" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        warn "Need sudo permission to install to $INSTALL_DIR"
        sudo mv "$BINARY_NAME" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi

    # Cleanup
    cd - > /dev/null
    rm -rf "$tmp_dir"

    info "Successfully installed $BINARY_NAME $version to $INSTALL_DIR"
    info ""
    info "Run '$BINARY_NAME --help' to get started"
}

# Check dependencies
check_dependencies() {
    local missing_deps=()

    for cmd in curl tar; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done

    if [ ${#missing_deps[@]} -ne 0 ]; then
        error "Missing required dependencies: ${missing_deps[*]}"
    fi
}

main() {
    echo "dbtree installer"
    echo "================"
    echo ""

    check_dependencies
    install_dbtree
}

main
