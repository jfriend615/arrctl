#!/bin/sh
# arrctl installer
# POSIX compliant - works with sh, dash, bash
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR  - Where to clone arrctl (default: ~/.arrctl)
#   BIN_DIR      - Where to create symlink (default: /usr/local/bin)

set -e

# Defaults
INSTALL_DIR="${INSTALL_DIR:-$HOME/.arrctl}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
REPO_URL="https://github.com/jfriend615/arrctl.git"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/arrctl"

# Colors (if terminal supports them)
if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
    RED=$(tput setaf 1 2>/dev/null || printf '')
    GREEN=$(tput setaf 2 2>/dev/null || printf '')
    YELLOW=$(tput setaf 3 2>/dev/null || printf '')
    BOLD=$(tput bold 2>/dev/null || printf '')
    RESET=$(tput sgr0 2>/dev/null || printf '')
else
    RED=""
    GREEN=""
    YELLOW=""
    BOLD=""
    RESET=""
fi

info() {
    printf "%s%s%s\n" "$BOLD" "$1" "$RESET"
}

success() {
    printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"
}

warn() {
    printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"
}

error() {
    printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2
}

die() {
    error "$1"
    exit 1
}

# Check for required dependencies
check_dependencies() {
    info "Checking dependencies..."
    
    missing=""
    
    if ! command -v curl >/dev/null 2>&1; then
        missing="$missing curl"
    fi
    
    if ! command -v jq >/dev/null 2>&1; then
        missing="$missing jq"
    fi
    
    if ! command -v git >/dev/null 2>&1; then
        missing="$missing git"
    fi
    
    if [ -n "$missing" ]; then
        die "Missing required dependencies:$missing"
    fi
    
    success "All dependencies found (curl, jq, git)"
}

# Clone or update the repository
install_repo() {
    if [ -d "$INSTALL_DIR" ]; then
        if [ -d "$INSTALL_DIR/.git" ]; then
            info "Updating existing installation at $INSTALL_DIR..."
            cd "$INSTALL_DIR"
            git pull --ff-only origin main 2>/dev/null || git pull origin main
            success "Updated to latest version"
        else
            die "$INSTALL_DIR exists but is not a git repository. Remove it first or set INSTALL_DIR."
        fi
    else
        info "Cloning arrctl to $INSTALL_DIR..."
        git clone "$REPO_URL" "$INSTALL_DIR"
        success "Cloned arrctl"
    fi
    
    # Ensure executable
    chmod +x "$INSTALL_DIR/bin/arrctl"
}

# Create symlink in BIN_DIR
create_symlink() {
    target="$INSTALL_DIR/bin/arrctl"
    link="$BIN_DIR/arrctl"
    
    info "Creating symlink at $link..."
    
    # Check if link already exists and points to correct target
    if [ -L "$link" ]; then
        existing_target=$(readlink "$link" 2>/dev/null || true)
        if [ "$existing_target" = "$target" ]; then
            success "Symlink already exists and is correct"
            return 0
        else
            warn "Symlink exists but points to $existing_target"
            # Try to update it
        fi
    elif [ -e "$link" ]; then
        die "$link exists and is not a symlink. Remove it manually first."
    fi
    
    # Create BIN_DIR if it doesn't exist
    if [ ! -d "$BIN_DIR" ]; then
        if mkdir -p "$BIN_DIR" 2>/dev/null; then
            : # success
        elif command -v sudo >/dev/null 2>&1; then
            sudo mkdir -p "$BIN_DIR"
        else
            die "Cannot create $BIN_DIR. Try running with sudo or set BIN_DIR to a writable location."
        fi
    fi
    
    # Try to create symlink, with sudo fallback
    if ln -sf "$target" "$link" 2>/dev/null; then
        success "Created symlink"
    elif command -v sudo >/dev/null 2>&1; then
        info "Need elevated permissions for $BIN_DIR..."
        if sudo ln -sf "$target" "$link"; then
            success "Created symlink (with sudo)"
        else
            die "Failed to create symlink even with sudo"
        fi
    else
        warn "Could not create symlink in $BIN_DIR"
        echo ""
        echo "Add this to your shell profile instead:"
        echo "  export PATH=\"\$PATH:$INSTALL_DIR/bin\""
        return 1
    fi
}

# Create config template
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

# Print final message
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
    echo "Documentation: https://github.com/jfriend615/arrctl"
}

# Main
main() {
    echo ""
    printf "%s%sarrctl installer%s\n" "$BOLD" "$GREEN" "$RESET"
    echo ""
    
    check_dependencies
    install_repo
    create_symlink
    create_config
    print_success
}

main "$@"
