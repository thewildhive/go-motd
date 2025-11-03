#!/bin/bash

# Install script for go-motd
# Downloads and installs the latest release from GitHub

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

# Default installation directory
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/motd.d}"

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${RESET} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${RESET} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${RESET} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${RESET} $1"
}

# Function to detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        freebsd)
            OS="freebsd"
            ;;
        *)
            print_error "Unsupported OS: $os"
            exit 1
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|arm)
            ARCH="arm"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    echo "${OS}-${ARCH}"
}

# Function to get latest release version
get_latest_version() {
    local api_url="https://api.github.com/repos/sst/go-motd/releases/latest"
    local version
    
    if command -v curl >/dev/null 2>&1; then
        version=$(curl -s "$api_url" | grep '"tag_name":' | sed -E 's/.*"tag_name": ?"v?([^"]+).*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        version=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"tag_name": ?"v?([^"]+).*/\1/')
    else
        print_error "Neither curl nor wget is available"
        exit 1
    fi
    
    if [ -z "$version" ]; then
        print_error "Failed to get latest version"
        exit 1
    fi
    
    echo "$version"
}

# Function to download and install motd
install_motd() {
    local version="$1"
    local platform="$2"
    local filename="motd-${version}-${platform}.tar.gz"
    local download_url="https://github.com/sst/go-motd/releases/download/v${version}/${filename}"
    local temp_dir=$(mktemp -d)
    
    print_info "Downloading motd v${version} for ${platform}..."
    
    # Download the release
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "${temp_dir}/${filename}" "$download_url"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "${temp_dir}/${filename}" "$download_url"
    else
        print_error "Neither curl nor wget is available"
        exit 1
    fi
    
    # Extract and install
    print_info "Extracting and installing..."
    cd "$temp_dir"
    tar -xzf "$filename"
    
    # Check if we have write permission to install directory
    if [ ! -w "$(dirname "$INSTALL_DIR")" ] && [ ! -w "$INSTALL_DIR" ]; then
        print_warning "No write permission to $INSTALL_DIR, trying with sudo..."
        if command -v sudo >/dev/null 2>&1; then
            sudo mkdir -p "$INSTALL_DIR"
            sudo cp motd "$INSTALL_DIR/"
            sudo chmod +x "$INSTALL_DIR/motd"
        else
            print_error "No write permission and sudo not available"
            exit 1
        fi
    else
        mkdir -p "$INSTALL_DIR"
        cp motd "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/motd"
    fi
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    print_success "motd v${version} installed to $INSTALL_DIR/motd"
}

# Function to create sample config
create_sample_config() {
    local config_file="$CONFIG_DIR/default.yml"
    
    if [ ! -f "$config_file" ]; then
        print_info "Creating sample configuration at $config_file..."
        
        if [ ! -w "$(dirname "$CONFIG_DIR")" ] && [ ! -w "$CONFIG_DIR" ]; then
            if command -v sudo >/dev/null 2>&1; then
                sudo mkdir -p "$CONFIG_DIR"
                sudo tee "$config_file" > /dev/null << 'EOF'
# motd configuration
# See https://github.com/sst/go-motd for more options

title: "Welcome!"
sections:
  - type: "text"
    content: "System Information"
  - type: "command"
    command: "uname -a"
  - type: "text"
    content: "Disk Usage:"
  - type: "command"
    command: "df -h"
EOF
            else
                print_warning "Cannot create config file without sudo permissions"
            fi
        else
            mkdir -p "$CONFIG_DIR"
            cat > "$config_file" << 'EOF'
# motd configuration
# See https://github.com/sst/go-motd for more options

title: "Welcome!"
sections:
  - type: "text"
    content: "System Information"
  - type: "command"
    command: "uname -a"
  - type: "text"
    content: "Disk Usage:"
  - type: "command"
    command: "df -h"
EOF
        fi
        
        print_success "Sample configuration created at $config_file"
    fi
}

# Main installation function
main() {
    print_info "Installing go-motd..."
    
    # Detect platform
    local platform=$(detect_platform)
    print_info "Detected platform: $platform"
    
    # Get latest version
    local version=$(get_latest_version)
    print_info "Latest version: v$version"
    
    # Install motd
    install_motd "$version" "$platform"
    
    # Create sample config
    create_sample_config
    
    print_success "Installation completed!"
    echo
    print_info "Usage:"
    echo "  $INSTALL_DIR/motd                    # Run with default config"
    echo "  $INSTALL_DIR/motd -c /path/to/config  # Run with custom config"
    echo
    print_info "Configuration directory: $CONFIG_DIR"
    print_info "For more information: https://github.com/sst/go-motd"
}

# Check if running with --help flag
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "go-motd installer"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -h, --help              Show this help message"
    echo "  -d, --dir DIR           Install to specific directory (default: /usr/local/bin)"
    echo "  -c, --config-dir DIR    Config directory (default: /etc/motd.d)"
    echo
    echo "Environment variables:"
    echo "  INSTALL_DIR              Installation directory"
    echo "  CONFIG_DIR               Configuration directory"
    echo
    echo "Examples:"
    echo "  $0                      # Install to /usr/local/bin"
    echo "  $0 -d ~/bin             # Install to ~/bin"
    echo "  INSTALL_DIR=~/bin $0    # Install to ~/bin using env var"
    exit 0
fi

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        -c|--config-dir)
            CONFIG_DIR="$2"
            shift 2
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main installation
main