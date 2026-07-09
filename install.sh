#!/bin/sh
# arrctl installer (release binary)
# POSIX compliant - works with sh, dash, bash
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
#
# Optional env vars:
#   BIN_DIR   - install location (default: /usr/local/bin)
#   VERSION   - release tag to install (default: latest)

set -e

BIN_DIR="${BIN_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"
REPO="jfriend615/arrctl"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/arrctl"

if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
    RED=$(tput setaf 1 2>/dev/null || printf '')
    GREEN=$(tput setaf 2 2>/dev/null || printf '')
    YELLOW=$(tput setaf 3 2>/dev/null || printf '')
    BOLD=$(tput bold 2>/dev/null || printf '')
    RESET=$(tput sgr0 2>/dev/null || printf '')
else
    RED=""; GREEN=""; YELLOW=""; BOLD=""; RESET=""
fi

info() { printf "%s%s%s\n" "$BOLD" "$1" "$RESET"; }
success() { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"; }
error() { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; }
die() { error "$1"; exit 1; }

cleanup_path() {
    if [ -n "${1:-}" ] && [ -e "$1" ]; then
        rm -f "$1"
    fi
    return 0
}

cleanup_path_sudo() {
    if [ -n "${1:-}" ]; then
        sudo rm -f "$1" >/dev/null 2>&1 || true
    fi
    return 0
}

check_dependencies() {
    info "Checking dependencies..."
    missing=""
    command -v curl >/dev/null 2>&1 || missing="$missing curl"
    command -v tar >/dev/null 2>&1 || missing="$missing tar"
    if [ -n "$missing" ]; then
        die "Missing required dependencies:$missing"
    fi
    success "All dependencies found"
}

detect_platform() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$os" in
        darwin|linux) ;;
        *) die "Unsupported OS: $os (supported: darwin, linux)" ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) die "Unsupported architecture: $arch (supported: amd64, arm64)" ;;
    esac

    PLATFORM_OS="$os"
    PLATFORM_ARCH="$arch"
}

resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        v_url="$(curl -sSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")"
        VERSION="${v_url##*/}"
        [ -n "$VERSION" ] || die "Unable to resolve latest release version"
    fi

    case "$VERSION" in
        v*) VERSION_TAG="$VERSION" ; VERSION_NO_V="${VERSION#v}" ;;
        *) VERSION_TAG="v$VERSION" ; VERSION_NO_V="$VERSION" ;;
    esac
}

download_and_install() {
    archive="arrctl_${VERSION_NO_V}_${PLATFORM_OS}_${PLATFORM_ARCH}.tar.gz"
    base_url="https://github.com/${REPO}/releases/download/${VERSION_TAG}"
    archive_url="${base_url}/${archive}"
    sums_url="${base_url}/SHA256SUMS"

    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT INT TERM

    info "Downloading ${archive}..."
    curl -fsSL "$archive_url" -o "$tmp_dir/$archive" || die "Failed to download release artifact: $archive_url"

    info "Downloading checksums..."
    curl -fsSL "$sums_url" -o "$tmp_dir/SHA256SUMS" || die "Failed to download checksums: $sums_url"

    info "Verifying checksum..."
    expected=$(grep "  ${archive}$" "$tmp_dir/SHA256SUMS" | awk '{print $1}')
    [ -n "$expected" ] || die "Could not find checksum for ${archive}"

    if command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')
    elif command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')
    else
        die "Need shasum or sha256sum to verify downloads"
    fi

    [ "$expected" = "$actual" ] || die "Checksum mismatch for ${archive}"
    success "Checksum verified"

    info "Extracting..."
    tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

    extracted_dir="$tmp_dir/arrctl_${VERSION_NO_V}_${PLATFORM_OS}_${PLATFORM_ARCH}"
    bin_src="$extracted_dir/arrctl"
    [ -f "$bin_src" ] || die "Extracted archive missing arrctl binary"

    if [ ! -d "$BIN_DIR" ]; then
        if mkdir -p "$BIN_DIR" 2>/dev/null; then :
        elif command -v sudo >/dev/null 2>&1; then sudo mkdir -p "$BIN_DIR"
        else die "Cannot create $BIN_DIR. Try setting BIN_DIR to a writable location."; fi
    fi

    target="$BIN_DIR/arrctl"
    target_dir=$(dirname "$target")

    tmp_target="$target_dir/.arrctl-install.$$"
    cleanup_path "$tmp_target"

    if cp "$bin_src" "$tmp_target" 2>/dev/null; then
        trap 'cleanup_path "$tmp_target"; rm -rf "$tmp_dir"' EXIT INT TERM
        chmod 755 "$tmp_target" || die "Cannot chmod +x $tmp_target"
        mv -f "$tmp_target" "$target" || die "Cannot replace $target"
    elif command -v sudo >/dev/null 2>&1; then
        tmp_target="$target_dir/.arrctl-install.$$"
        cleanup_path_sudo "$tmp_target"
        trap 'rm -rf "$tmp_dir"' EXIT INT TERM
        sudo cp "$bin_src" "$tmp_target" || die "Cannot stage $target"
        sudo chmod 755 "$tmp_target" || {
            cleanup_path_sudo "$tmp_target"
            die "Cannot chmod +x $tmp_target"
        }
        sudo mv -f "$tmp_target" "$target" || {
            cleanup_path_sudo "$tmp_target"
            die "Cannot replace $target"
        }
    else
        die "Cannot write to $target"
    fi

    success "Installed arrctl ${VERSION_TAG} to ${target}"
}

create_config() {
    config_file="$CONFIG_DIR/config.json"
    if [ -f "$config_file" ]; then
        success "Config file already exists at $config_file"
        return 0
    fi

    info "Creating config template at $config_file..."
    mkdir -p "$CONFIG_DIR"

    cat > "$config_file" <<'EOF'
{
  "sonarr": {
    "url": "http://localhost:8989",
    "api_key": "your-sonarr-api-key-here"
  },
  "radarr": {
    "url": "http://localhost:7878",
    "api_key": "your-radarr-api-key-here"
  },
  "overseerr": {
    "url": "http://localhost:5055",
    "api_key": "your-overseerr-api-key-here"
  },
  "tautulli": {
    "url": "http://localhost:8181",
    "api_key": "your-tautulli-api-key-here"
  }
}
EOF

    success "Created config template"
}

print_success() {
    echo ""
    printf "%s%s========================================%s\n" "$GREEN" "$BOLD" "$RESET"
    printf "%s%s arrctl installed successfully! %s\n" "$GREEN" "$BOLD" "$RESET"
    printf "%s%s========================================%s\n" "$GREEN" "$BOLD" "$RESET"
    echo ""
    echo "Next steps:"
    echo "  1. Edit your config file with API keys:"
    echo "     ${CONFIG_DIR}/config.json"
    echo ""
    echo "  2. Verify installation:"
    echo "     arrctl --version"
    echo ""
    echo "  3. Get started:"
    echo "     arrctl --help"
    echo ""
    echo "Documentation: https://github.com/${REPO}"
}

main() {
    echo ""
    printf "%s%sarrctl installer%s\n" "$BOLD" "$GREEN" "$RESET"
    echo ""

    check_dependencies
    detect_platform
    resolve_version
    download_and_install
    create_config
    print_success
}

main "$@"
