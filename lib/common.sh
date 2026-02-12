#!/bin/sh
# common.sh - Shared utilities for arrctl
# POSIX compliant - tested with dash

# Default config path
DEFAULT_CONFIG="${HOME}/.config/arrctl/config.json"

# Global output settings (set by parse_service_args)
OUTPUT_FORMAT="${OUTPUT_FORMAT:-auto}"
QUIET_MODE="${QUIET_MODE:-0}"

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

# Make API request with proper HTTP code handling
# Usage: api_request "GET" "/api/v3/series" [data]
# Outputs: JSON response to stdout
# Sets: HTTP_CODE global with response status
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
    
    # Use -w to append HTTP code on a new line, then parse it out
    case "$_method" in
        GET)
            _response="$(curl -s -X GET \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Accept: application/json" \
                -w "\n%{http_code}" \
                "$_url")"
            ;;
        POST)
            _response="$(curl -s -X POST \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Content-Type: application/json" \
                -H "Accept: application/json" \
                -d "$_data" \
                -w "\n%{http_code}" \
                "$_url")"
            ;;
        PUT)
            _response="$(curl -s -X PUT \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Content-Type: application/json" \
                -H "Accept: application/json" \
                -d "$_data" \
                -w "\n%{http_code}" \
                "$_url")"
            ;;
        DELETE)
            _response="$(curl -s -X DELETE \
                -H "X-Api-Key: ${SERVICE_API_KEY}" \
                -H "Accept: application/json" \
                -w "\n%{http_code}" \
                "$_url")"
            ;;
        *)
            die "Unknown HTTP method: $_method"
            ;;
    esac
    
    # Extract HTTP code from last line
    HTTP_CODE="$(printf '%s' "$_response" | tail -n1)"
    _body="$(printf '%s' "$_response" | sed '$d')"
    
    # Handle HTTP errors
    case "$HTTP_CODE" in
        2*)
            # Success - output body
            printf '%s\n' "$_body"
            ;;
        401)
            die "Authentication failed - check API key"
            ;;
        404)
            die "Not found: $_endpoint"
            ;;
        *)
            if [ -n "$_body" ]; then
                die "API request failed (HTTP $HTTP_CODE): $_body"
            else
                die "API request failed (HTTP $HTTP_CODE)"
            fi
            ;;
    esac
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

# Parse service-level args for --format and --quiet
# Sets: OUTPUT_FORMAT, QUIET_MODE globals
# Returns remaining args via _REMAINING_ARGS
# Usage: parse_service_args "$@"; eval "set -- $_REMAINING_ARGS"
parse_service_args() {
    _REMAINING_ARGS=""
    OUTPUT_FORMAT="auto"
    QUIET_MODE=0
    
    while [ $# -gt 0 ]; do
        case "$1" in
            --format)
                if [ -z "${2:-}" ]; then
                    die "--format requires a value (json|table|auto)"
                fi
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            --format=*)
                OUTPUT_FORMAT="${1#--format=}"
                shift
                ;;
            -q|--quiet)
                QUIET_MODE=1
                shift
                ;;
            *)
                _REMAINING_ARGS="$_REMAINING_ARGS '$1'"
                shift
                ;;
        esac
    done
    
    # Validate format
    case "$OUTPUT_FORMAT" in
        json|table|auto) ;;
        *) die "Invalid format: $OUTPUT_FORMAT (use json|table|auto)" ;;
    esac
}

# Format JSON as table using jq
# Usage: echo "$json" | format_table "headers" "jq_expression"
# Example: echo "$json" | format_table "ID|Title|Year" '.[] | [.id, .title, .year]'
format_table() {
    _headers="$1"
    _jq_expr="$2"
    
    check_deps jq
    
    # Determine if we should use table format
    _use_table=0
    case "$OUTPUT_FORMAT" in
        table) _use_table=1 ;;
        auto) [ -t 1 ] && _use_table=1 ;;
    esac
    
    if [ "$_use_table" -eq 1 ]; then
        # Print headers
        printf '%s\n' "$_headers" | tr '|' '\t'
        printf '%s\n' "$_headers" | sed 's/[^|]/-/g' | tr '|' '\t'
        # Print rows
        jq -r "$_jq_expr | @tsv"
    else
        # Pass through as JSON
        jq '.'
    fi
}

# URL encode a string (POSIX compliant)
# Usage: url_encode "search term"
url_encode() {
    _string="$1"
    printf '%s' "$_string" | jq -sRr @uri
}

# Get config value with fallback
# Usage: get_config_value "sonarr" "defaults.qualityProfile" "default_value"
get_config_value() {
    _service="$1"
    _key="$2"
    _default="${3:-}"
    
    _config_file="${ARRCTL_CONFIG:-$DEFAULT_CONFIG}"
    
    if [ -f "$_config_file" ]; then
        _value="$(jq -r ".${_service}.${_key} // empty" "$_config_file" 2>/dev/null)"
        if [ -n "$_value" ]; then
            printf '%s' "$_value"
            return 0
        fi
    fi
    
    printf '%s' "$_default"
}
