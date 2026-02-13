#!/bin/sh
# tautulli.sh - Tautulli (Plex stats) CLI module
# POSIX compliant - tested with dash

# Main entry point for tautulli subcommand
tautulli_main() {
    # Handle no arguments or help before loading config
    if [ $# -eq 0 ]; then
        tautulli_help
        return 0
    fi
    
    case "$1" in
        -h|--help|help)
            tautulli_help
            return 0
            ;;
    esac
    
    # Now load config and parse args for actual commands
    load_config "tautulli"
    parse_service_args "$@"
    eval "set -- $_REMAINING_ARGS"
    
    case "${1:-}" in
        now)
            shift
            tautulli_now "$@"
            ;;
        *)
            die "Unknown tautulli command: $1. Use 'arrctl tautulli help' for usage."
            ;;
    esac
}

# Show help for tautulli commands
tautulli_help() {
    cat <<EOF
arrctl tautulli - View Plex activity via Tautulli

Usage: arrctl tautulli <command> [options]

Commands:
    now              Show who is currently streaming
    help             Show this help message

Options:
    --format FORMAT  Output format: json|table|auto (default: auto)
    -q, --quiet      Exit code only (0=active, 1=none)

Exit Codes:
    0                Active streams found (or help displayed)
    1                No active streams

Examples:
    arrctl tautulli now
    arrctl tautulli now --format=json
    arrctl tautulli now --quiet && echo "Someone is watching" || echo "No active streams"
EOF
}

# Show current streaming activity
# Usage: tautulli_now
tautulli_now() {
    check_deps curl jq
    
    # Tautulli uses query-param auth (NOT X-Api-Key header like *arr services)
    _url="${SERVICE_URL%/}/api/v2?cmd=get_activity&apikey=${SERVICE_API_KEY}"
    _response="$(curl -s "$_url")"
    
    # Check for API errors
    _result="$(printf '%s' "$_response" | jq -r '.response.result // "error"')"
    if [ "$_result" != "success" ]; then
        _message="$(printf '%s' "$_response" | jq -r '.response.message // "Unknown error"')"
        die "Tautulli API error: $_message"
    fi
    
    # Extract sessions from .response.data.sessions (wrapped response)
    _sessions="$(printf '%s' "$_response" | jq '.response.data.sessions // []')"
    _count="$(printf '%s' "$_sessions" | jq 'length')"
    
    # Handle no active streams
    if [ "$_count" -eq 0 ]; then
        if [ "$QUIET_MODE" -eq 0 ]; then
            info "No active streams"
        fi
        return 1
    fi
    
    # Quiet mode - just exit with success
    if [ "$QUIET_MODE" -eq 1 ]; then
        return 0
    fi
    
    # Format output
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        printf '%s\n' "$_sessions" | jq '.'
    else
        # Table format with fallback field names
        printf '%s\n' "$_sessions" | format_table \
            "User|Title|Progress|Quality|Transcode|State" \
            '.[] | [
                (.user // .friendly_name // "Unknown"),
                (.full_title // .title // "Unknown"),
                ((.progress_percent // 0 | tostring) + "%"),
                (.video_full_resolution // .quality_profile // "Unknown"),
                (.transcode_decision // "Unknown"),
                (.state // "Unknown")
            ]'
    fi
}
