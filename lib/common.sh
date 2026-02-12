#!/bin/sh
# common.sh - Shared utilities for arrctl
# POSIX compliant - tested with dash

# Default config path
DEFAULT_CONFIG="${HOME}/.config/arrctl/config.json"

# Print error message and exit
# Usage: die "error message"
die() {
    printf "Error: %s\n" "$1" >&2
    exit 1
}

# Print warning message to stderr
# Usage: warn "warning message"
warn() {
    printf "Warning: %s\n" "$1" >&2
}

# Print info message to stderr (for non-data output)
# Usage: info "info message"
info() {
    printf "%s\n" "$1" >&2
}

# Check if required commands are available
# Usage: check_deps curl jq
check_deps() {
    for cmd in "$@"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            die "Required command not found: $cmd"
        fi
    done
}

# Load configuration from file or environment
# Sets: SERVICE_URL, SERVICE_API_KEY for the given service
# Usage: load_config "sonarr"
load_config() {
    _service="$1"
    _service_upper="$(printf '%s' "$_service" | tr '[:lower:]' '[:upper:]')"
    
    # Check for config file (env var takes precedence)
    _config_file="${ARRCTL_CONFIG:-$DEFAULT_CONFIG}"
    
    # Try environment variables first
    eval "_url_var=\"\${${_service_upper}_URL:-}\""
    eval "_key_var=\"\${${_service_upper}_API_KEY:-}\""
    
    if [ -n "$_url_var" ] && [ -n "$_key_var" ]; then
        SERVICE_URL="$_url_var"
        SERVICE_API_KEY="$_key_var"
        return 0
    fi
    
    # Fall back to config file
    if [ -f "$_config_file" ]; then
        check_deps jq
        
        # Validate JSON
        if ! jq empty "$_config_file" 2>/dev/null; then
            die "Invalid JSON in config file: $_config_file"
        fi
        
        SERVICE_URL="$(jq -r ".${_service}.url // empty" "$_config_file")"
        SERVICE_API_KEY="$(jq -r ".${_service}.api_key // empty" "$_config_file")"
        
        if [ -z "$SERVICE_URL" ] || [ -z "$SERVICE_API_KEY" ]; then
            die "Missing ${_service} configuration. Set ${_service_upper}_URL and ${_service_upper}_API_KEY environment variables or configure in $_config_file"
        fi
        
        return 0
    fi
    
    die "No configuration found. Set environment variables or create $_config_file"
}

# Make API request
# Usage: api_request "GET" "/api/v3/series" [data]
# Outputs: JSON response to stdout
api_request() {
    _method="$1"
    _endpoint="$2"
    _data="${3:-}"
    
    check_deps curl
    
    if [ -z "$SERVICE_URL" ] || [ -z "$SERVICE_API_KEY" ]; then
        die "Service not configured. Call load_config first."
    fi
    
    # Build URL (handle trailing slash in SERVICE_URL)
    _base_url="${SERVICE_URL%/}"
    _url="${_base_url}${_endpoint}"
    
    case "$_method" in
        GET)
            curl -s -f -X GET \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Accept: application/json" \
                "$_url"
            ;;
        POST)
            curl -s -f -X POST \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Content-Type: application/json" \
                -H "Accept: application/json" \
                -d "$_data" \
                "$_url"
            ;;
        PUT)
            curl -s -f -X PUT \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Content-Type: application/json" \
                -H "Accept: application/json" \
                -d "$_data" \
                "$_url"
            ;;
        DELETE)
            curl -s -f -X DELETE \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Accept: application/json" \
                "$_url"
            ;;
        *)
            die "Unknown HTTP method: $_method"
            ;;
    esac
    
    _exit_code=$?
    if [ $_exit_code -ne 0 ]; then
        die "API request failed (curl exit code: $_exit_code)"
    fi
}

# Format JSON output for human reading (when stdout is terminal)
# Usage: echo "$json" | format_output
format_output() {
    if [ -t 1 ]; then
        jq '.'
    else
        cat
    fi
}

# Parse common options and return remaining args
# Sets: ARRCTL_CONFIG if --config is provided
# Usage: eval "set -- $(parse_common_opts "$@")"
parse_common_opts() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --config)
                if [ -z "${2:-}" ]; then
                    die "--config requires a file path"
                fi
                ARRCTL_CONFIG="$2"
                export ARRCTL_CONFIG
                shift 2
                ;;
            --config=*)
                ARRCTL_CONFIG="${1#--config=}"
                export ARRCTL_CONFIG
                shift
                ;;
            *)
                printf " %s" "'$1'"
                shift
                ;;
        esac
    done
}
