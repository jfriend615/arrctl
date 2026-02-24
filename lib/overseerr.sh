#!/bin/sh
# overseerr.sh - Overseerr (media requests) CLI module
# POSIX compliant - tested with dash

# Main entry point for overseerr subcommand
overseerr_main() {
    # Handle no arguments or help before loading config
    if [ $# -eq 0 ]; then
        overseerr_help
        return 0
    fi
    
    case "$1" in
        -h|--help|help)
            overseerr_help
            return 0
            ;;
    esac
    
    # Now load config and parse args for actual commands
    load_config "overseerr"
    parse_service_args "$@"
    eval "set -- $_REMAINING_ARGS"
    
    case "${1:-}" in
        pending)
            shift
            overseerr_pending "$@"
            ;;
        approve)
            shift
            overseerr_approve "$@"
            ;;
        deny|decline)
            shift
            overseerr_deny "$@"
            ;;
        *)
            die "Unknown overseerr command: $1. Use 'arrctl overseerr help' for usage."
            ;;
    esac
}

# Show help for overseerr commands
overseerr_help() {
    cat <<EOF
arrctl overseerr - Manage media requests via Overseerr

Usage: arrctl overseerr <command> [options]

Commands:
    pending            List pending requests awaiting approval
    approve <id>       Approve a pending request
    deny <id>          Deny/decline a pending request
    help               Show this help message

Options:
    --format FORMAT    Output format: json|table|auto (default: auto)
    --message MSG      Message to include when approving (optional)
    --reason MSG       Reason for denial when declining (optional)

Examples:
    arrctl overseerr pending
    arrctl overseerr pending --format=json
    arrctl overseerr approve 123
    arrctl overseerr approve 124 --message "Adding tonight"
    arrctl overseerr deny 125 --reason "Already available"
    arrctl overseerr decline 126
EOF
}

# Resolve request title from media type + TMDB id
# Usage: overseerr_resolve_request_title <type> <tmdb_id>
overseerr_resolve_request_title() {
    _type="$1"
    _tmdb_id="$2"

    case "$_type" in
        movie|tv) ;;
        *)
            printf 'Unknown'
            return 0
            ;;
    esac

    if [ -z "$_tmdb_id" ] || [ "$_tmdb_id" = "null" ]; then
        printf 'Unknown'
        return 0
    fi

    _detail="$(api_request GET "/api/v1/${_type}/${_tmdb_id}")"
    printf '%s' "$_detail" | jq -r '.title // .name // "Unknown"'
}

# List pending requests
# Usage: overseerr_pending
overseerr_pending() {
    check_deps curl jq

    _response="$(api_request GET "/api/v1/request?filter=pending&take=100")"

    # Extract results array
    _results="$(printf '%s' "$_response" | jq '.results // []')"
    _count="$(printf '%s' "$_results" | jq 'length')"

    # Handle no pending requests
    if [ "$_count" -eq 0 ]; then
        info "No pending requests"
        return 0
    fi

    # Format output
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        printf '%s\n' "$_results" | jq '.'
    else
        # Table format: ID, User, Title, Type, Date
        # Note: request payload includes media identifiers but not a title,
        # so we resolve titles from /movie/<tmdbId> or /tv/<tmdbId>.
        _use_table=0
        case "$OUTPUT_FORMAT" in
            table) _use_table=1 ;;
            auto) [ -t 1 ] && _use_table=1 ;;
        esac

        if [ "$_use_table" -eq 1 ]; then
            if command -v column >/dev/null 2>&1; then
                {
                    printf '%s\n' "ID|User|Title|Type|Date"
                    printf '%s\n' "$_results" | jq -c '.[]' | while IFS= read -r _req; do
                        _id="$(printf '%s' "$_req" | jq -r '.id')"
                        _user="$(printf '%s' "$_req" | jq -r '.requestedBy.displayName // "Unknown"')"
                        _type="$(printf '%s' "$_req" | jq -r '.type // .media.mediaType // "unknown"')"
                        _tmdb_id="$(printf '%s' "$_req" | jq -r '.media.tmdbId // empty')"
                        _title="$(printf '%s' "$_req" | jq -r '.media.title // .media.name // .title // empty')"
                        if [ -z "$_title" ] || [ "$_title" = "null" ]; then
                            _title="$(overseerr_resolve_request_title "$_type" "$_tmdb_id")"
                        fi
                        _date="$(printf '%s' "$_req" | jq -r '(.createdAt // "") | split("T")[0]')"
                        printf '%s|%s|%s|%s|%s\n' "$_id" "$_user" "$_title" "$_type" "$_date"
                    done
                } | column -t -s '|'
            else
                printf '%s\n' "ID\tUser\tTitle\tType\tDate"
                printf '%s\n' "$_results" | jq -c '.[]' | while IFS= read -r _req; do
                    _id="$(printf '%s' "$_req" | jq -r '.id')"
                    _user="$(printf '%s' "$_req" | jq -r '.requestedBy.displayName // "Unknown"')"
                    _type="$(printf '%s' "$_req" | jq -r '.type // .media.mediaType // "unknown"')"
                    _tmdb_id="$(printf '%s' "$_req" | jq -r '.media.tmdbId // empty')"
                    _title="$(printf '%s' "$_req" | jq -r '.media.title // .media.name // .title // empty')"
                    if [ -z "$_title" ] || [ "$_title" = "null" ]; then
                        _title="$(overseerr_resolve_request_title "$_type" "$_tmdb_id")"
                    fi
                    _date="$(printf '%s' "$_req" | jq -r '(.createdAt // "") | split("T")[0]')"
                    printf '%s\t%s\t%s\t%s\t%s\n' "$_id" "$_user" "$_title" "$_type" "$_date"
                done
            fi
        else
            # Non-table fallback: emit structured JSON with resolved display fields.
            printf '%s\n' "$_results" | jq -c '.[]' | while IFS= read -r _req; do
                _type="$(printf '%s' "$_req" | jq -r '.type // .media.mediaType // "unknown"')"
                _tmdb_id="$(printf '%s' "$_req" | jq -r '.media.tmdbId // empty')"
                _title="$(printf '%s' "$_req" | jq -r '.media.title // .media.name // .title // empty')"
                if [ -z "$_title" ] || [ "$_title" = "null" ]; then
                    _title="$(overseerr_resolve_request_title "$_type" "$_tmdb_id")"
                fi

                printf '%s' "$_req" | jq \
                    --arg user "$(printf '%s' "$_req" | jq -r '.requestedBy.displayName // "Unknown"')" \
                    --arg title "$_title" \
                    '. + {requestUser: $user, requestTitle: $title}'
            done | jq -s '.'
        fi
    fi
}

# Approve a pending request
# Usage: overseerr_approve <id> [--message MSG]
overseerr_approve() {
    check_deps curl jq
    
    _id=""
    _message=""
    
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --message)
                if [ -z "${2:-}" ]; then
                    die "--message requires a value"
                fi
                _message="$2"
                shift 2
                ;;
            --message=*)
                _message="${1#--message=}"
                shift
                ;;
            -*)
                die "Unknown option: $1"
                ;;
            *)
                if [ -z "$_id" ]; then
                    _id="$1"
                else
                    die "Unexpected argument: $1"
                fi
                shift
                ;;
        esac
    done
    
    # Validate ID
    if [ -z "$_id" ]; then
        die "Request ID required. Usage: arrctl overseerr approve <id>"
    fi
    
    # Build request body
    if [ -n "$_message" ]; then
        _body="$(printf '{"message":"%s"}' "$_message")"
    else
        _body="{}"
    fi
    
    # Make API request (api_request handles errors)
    api_request POST "/api/v1/request/${_id}/approve" "$_body" >/dev/null
    
    info "Approved request $_id"
}

# Deny/decline a pending request
# Usage: overseerr_deny <id> [--reason MSG]
overseerr_deny() {
    check_deps curl jq
    
    _id=""
    _reason=""
    
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --reason)
                if [ -z "${2:-}" ]; then
                    die "--reason requires a value"
                fi
                _reason="$2"
                shift 2
                ;;
            --reason=*)
                _reason="${1#--reason=}"
                shift
                ;;
            -*)
                die "Unknown option: $1"
                ;;
            *)
                if [ -z "$_id" ]; then
                    _id="$1"
                else
                    die "Unexpected argument: $1"
                fi
                shift
                ;;
        esac
    done
    
    # Validate ID
    if [ -z "$_id" ]; then
        die "Request ID required. Usage: arrctl overseerr deny <id>"
    fi
    
    # Build request body
    if [ -n "$_reason" ]; then
        _body="$(printf '{"reason":"%s"}' "$_reason")"
    else
        _body="{}"
    fi
    
    # Make API request (api_request handles errors)
    api_request POST "/api/v1/request/${_id}/decline" "$_body" >/dev/null
    
    info "Declined request $_id"
}
