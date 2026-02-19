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
        stale)
            shift
            tautulli_stale "$@"
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
    stale            Show stale media candidates (rarely watched + old)
    help             Show this help message

Global Options:
    --format FORMAT  Output format: json|table|auto (default: auto)
    -q, --quiet      Exit code only (supported by 'now')

Stale Options:
    --library VALUE  Filter by library name or section ID
    --min-days N     Minimum days since last played (default: 180)
    --max-plays N    Maximum play count to include (default: 2)
    --min-size-gb N  Minimum file size in GiB (default: 1)
    --limit N        Limit results (default: 50)
    --json           Shortcut for JSON output (same as --format=json)

Exit Codes:
    now:   0 active streams, 1 no active streams
    stale: 0 matches found, 1 no matches, 2 error

Examples:
    arrctl tautulli now
    arrctl tautulli stale
    arrctl tautulli stale --library Movies --min-days 365 --max-plays 1 --min-size-gb 4
    arrctl tautulli stale --json
EOF
}

# Tautulli API helper for query-param based API
# Usage: _tautulli_api "cmd=get_activity&foo=bar"
_tautulli_api() {
    _query="$1"
    _url="${SERVICE_URL%/}/api/v2?apikey=${SERVICE_API_KEY}&${_query}"
    curl -s "$_url"
}

# Show current streaming activity
# Usage: tautulli_now
tautulli_now() {
    check_deps curl jq

    _response="$(_tautulli_api "cmd=get_activity")"

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

# Show stale media candidates (read-only)
# Exit codes: 0 matches, 1 no matches, 2 errors
tautulli_stale() {
    check_deps curl jq

    _library=""
    _min_days=180
    _max_plays=2
    _min_size_gb=1
    _limit=50

    while [ $# -gt 0 ]; do
        case "$1" in
            --library)
                if [ -z "${2:-}" ]; then
                    printf 'Error: --library requires a value\n' >&2
                    return 2
                fi
                _library="$2"
                shift 2
                ;;
            --library=*)
                _library="${1#--library=}"
                shift
                ;;
            --min-days)
                if [ -z "${2:-}" ]; then
                    printf 'Error: --min-days requires a number\n' >&2
                    return 2
                fi
                _min_days="$2"
                shift 2
                ;;
            --min-days=*)
                _min_days="${1#--min-days=}"
                shift
                ;;
            --max-plays)
                if [ -z "${2:-}" ]; then
                    printf 'Error: --max-plays requires a number\n' >&2
                    return 2
                fi
                _max_plays="$2"
                shift 2
                ;;
            --max-plays=*)
                _max_plays="${1#--max-plays=}"
                shift
                ;;
            --min-size-gb)
                if [ -z "${2:-}" ]; then
                    printf 'Error: --min-size-gb requires a number\n' >&2
                    return 2
                fi
                _min_size_gb="$2"
                shift 2
                ;;
            --min-size-gb=*)
                _min_size_gb="${1#--min-size-gb=}"
                shift
                ;;
            --limit)
                if [ -z "${2:-}" ]; then
                    printf 'Error: --limit requires a number\n' >&2
                    return 2
                fi
                _limit="$2"
                shift 2
                ;;
            --limit=*)
                _limit="${1#--limit=}"
                shift
                ;;
            --json)
                OUTPUT_FORMAT="json"
                shift
                ;;
            -h|--help)
                tautulli_help
                return 0
                ;;
            *)
                printf 'Error: Unknown option: %s\n' "$1" >&2
                return 2
                ;;
        esac
    done

    # Validate numeric inputs
    case "$_min_days" in ''|*[!0-9]*) printf 'Error: --min-days must be a non-negative integer\n' >&2; return 2 ;; esac
    case "$_max_plays" in ''|*[!0-9]*) printf 'Error: --max-plays must be a non-negative integer\n' >&2; return 2 ;; esac
    case "$_limit" in ''|*[!0-9]*) printf 'Error: --limit must be a positive integer\n' >&2; return 2 ;; esac
    if [ "$_limit" -eq 0 ]; then
        printf 'Error: --limit must be greater than 0\n' >&2
        return 2
    fi

    # Validate min-size-gb as numeric (integer or decimal)
    if ! printf '%s' "$_min_size_gb" | grep -Eq '^[0-9]+([.][0-9]+)?$'; then
        printf 'Error: --min-size-gb must be a non-negative number\n' >&2
        return 2
    fi

    _libraries_response="$(_tautulli_api "cmd=get_libraries")"
    _libraries_result="$(printf '%s' "$_libraries_response" | jq -r '.response.result // "error"')"
    if [ "$_libraries_result" != "success" ]; then
        _message="$(printf '%s' "$_libraries_response" | jq -r '.response.message // "Unknown error"')"
        printf 'Error: Tautulli API error (get_libraries): %s\n' "$_message" >&2
        return 2
    fi

    _libraries="$(printf '%s' "$_libraries_response" | jq '.response.data // []')"
    _selected_libraries="$_libraries"

    if [ -n "$_library" ]; then
        _selected_libraries="$(printf '%s' "$_libraries" | jq --arg lib "$_library" '[.[] | select((.section_id|tostring)==$lib or ((.section_name // "") | ascii_downcase)==($lib|ascii_downcase))]')"
        _selected_count="$(printf '%s' "$_selected_libraries" | jq 'length')"
        if [ "$_selected_count" -eq 0 ]; then
            printf 'Error: Library not found: %s\n' "$_library" >&2
            return 2
        fi
    fi

    _all_items='[]'
    _ids="$(printf '%s' "$_selected_libraries" | jq -r '.[].section_id')"

    for _section_id in $_ids; do
        _media_response="$(_tautulli_api "cmd=get_library_media_info&section_id=${_section_id}&length=10000")"
        _media_result="$(printf '%s' "$_media_response" | jq -r '.response.result // "error"')"
        if [ "$_media_result" != "success" ]; then
            _message="$(printf '%s' "$_media_response" | jq -r '.response.message // "Unknown error"')"
            printf 'Error: Tautulli API error (get_library_media_info section_id=%s): %s\n' "$_section_id" "$_message" >&2
            return 2
        fi

        _section_name="$(printf '%s' "$_selected_libraries" | jq -r --arg sid "$_section_id" '.[] | select((.section_id|tostring)==$sid) | .section_name' | head -n1)"
        _items="$(printf '%s' "$_media_response" | jq --arg lib_name "${_section_name:-Unknown}" '.response.data.data // [] | map(.library_name = (.library_name // .section_name // $lib_name // "Unknown"))')"
        _all_items="$(printf '%s\n%s\n' "$_all_items" "$_items" | jq -s 'add')"
    done

    _now_epoch="$(date +%s)"
    _min_size_bytes="$(awk "BEGIN {printf \"%.0f\", $_min_size_gb * 1073741824}")"

    _result_json="$(printf '%s' "$_all_items" | jq \
        --argjson now "$_now_epoch" \
        --argjson min_days "$_min_days" \
        --argjson max_plays "$_max_plays" \
        --argjson min_size "$_min_size_bytes" \
        --argjson limit "$_limit" '
        [ .[]
          | .file_size = (try ((.file_size // 0) | tonumber) catch 0)
          | .play_count = (try ((.play_count // 0) | tonumber) catch 0)
          | .last_played = (try ((.last_played // 0) | tonumber) catch 0)
          | .added_at = (try ((.added_at // 0) | tonumber) catch 0)
          | .days_since_last_played = (if .last_played > 0 then ((($now - .last_played) / 86400) | floor) else 999999 end)
          | .size_gb = ((.file_size / 1073741824) * 100 | floor) / 100
          | select(.file_size >= $min_size)
          | select(.play_count <= $max_plays)
          | select(.days_since_last_played >= $min_days)
          | .stale_score = ((.size_gb * 0.6) + (.days_since_last_played / 365 * 0.3) + ((($max_plays + 1) - .play_count) * 0.1))
        ]
        | sort_by(-.stale_score, -.file_size, .last_played)
        | .[:$limit]
        ' )"

    _count="$(printf '%s' "$_result_json" | jq 'length')"
    if [ "$_count" -eq 0 ]; then
        info "No stale candidates found"
        return 1
    fi

    _use_table=0
    case "$OUTPUT_FORMAT" in
        table) _use_table=1 ;;
        auto) [ -t 1 ] && _use_table=1 ;;
    esac

    if [ "$_use_table" -eq 1 ]; then
        printf '%s\n' "$_result_json" | format_table \
            "Library|Type|Title|Size(GB)|Plays|Last Played|Date Added" \
            '.[] | [
                (.library_name // "Unknown"),
                (.media_type // "unknown"),
                (.title // .sort_title // "Unknown"),
                (.size_gb | tostring),
                (.play_count | tostring),
                (if (.last_played // 0) > 0 then (.last_played | strftime("%Y-%m-%d")) else "Never" end),
                (if (.added_at // 0) > 0 then (.added_at | strftime("%Y-%m-%d")) else "Unknown" end)
            ]'
    else
        printf '%s\n' "$_result_json" | jq '.'
    fi

    if [ "$_use_table" -eq 1 ]; then
        _total_gb="$(printf '%s' "$_result_json" | jq '[.[].file_size] | add // 0 | . / 1073741824')"
        info "$_count stale candidate(s) | total size: $(printf '%.2f' "$_total_gb") GiB"
    fi

    return 0
}
