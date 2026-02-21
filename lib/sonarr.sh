#!/bin/sh
# sonarr.sh - Sonarr service commands for arrctl
# POSIX compliant - tested with dash

# Show help for sonarr commands
sonarr_help() {
    cat <<EOF
arrctl sonarr - Manage Sonarr (TV shows)

Usage: arrctl sonarr <command> [options]

Commands:
    list              List all series in library
    search <term>     Search for series by name
    add               Add a series to library
    info              Show detailed series information
    delete            Delete a series from library
    calendar          Show upcoming episodes

List Options:
    --monitored       Show only monitored series
    --unmonitored     Show only unmonitored series
    --format FORMAT   Output format: json|table|auto (default: auto)
    -q, --quiet       Suppress non-essential output

Search Options:
    --limit N         Limit results (default: 10)
    --format FORMAT   Output format: json|table|auto (default: auto)

Add Options:
    --id TVDB_ID      TVDB ID of series to add (required)
    --quality NAME    Quality profile name or ID
    --root PATH       Root folder path
    --search          Start search for episodes after adding
    --monitored       Monitor the series (default: true)
    --no-monitored    Don't monitor the series

Info Options:
    --id ID           Sonarr series ID (exact match)
    --name NAME       Series name (partial match)
    --format FORMAT   Output format: json|table|auto (default: auto)

Delete Options:
    --id ID           Sonarr series ID to delete (required)
    --delete-files    Also delete series files from disk
    --add-exclusion   Add import list exclusion for this series
    --yes             Skip confirmation prompt

Calendar Options:
    --days N          Show next N days (default: 7)
    --start DATE      Start date (YYYY-MM-DD)
    --end DATE        End date (YYYY-MM-DD)
    --format FORMAT   Output format: json|table|auto (default: auto)

Examples:
    arrctl sonarr list
    arrctl sonarr list --monitored --format table
    arrctl sonarr search "Breaking Bad"
    arrctl sonarr search "The Office" --limit 5
    arrctl sonarr info --name "Severance"
    arrctl sonarr info --id 42 --format json
    arrctl sonarr add --id 81189 --quality "HD-1080p" --search
    arrctl sonarr delete --id 42 --delete-files
    arrctl sonarr calendar --days 14

EOF
}

# Main entry point for sonarr commands
sonarr_main() {
    # Handle no arguments or help before loading config
    if [ $# -eq 0 ]; then
        sonarr_help
        return 0
    fi
    
    # Check for help flags first (before config load)
    for _arg in "$@"; do
        case "$_arg" in
            -h|--help|help)
                sonarr_help
                return 0
                ;;
        esac
    done
    
    # Load configuration (only needed for actual commands)
    load_config "sonarr"
    
    # Parse service-level args
    parse_service_args "$@"
    eval "set -- $_REMAINING_ARGS"
    
    case "$1" in
        list)
            shift
            sonarr_list "$@"
            ;;
        search)
            shift
            sonarr_search "$@"
            ;;
        add)
            shift
            sonarr_add "$@"
            ;;
        info)
            shift
            sonarr_info "$@"
            ;;
        delete)
            shift
            sonarr_delete "$@"
            ;;
        calendar)
            shift
            sonarr_calendar "$@"
            ;;
        *)
            die "Unknown sonarr command: $1. Use 'arrctl sonarr --help' for usage."
            ;;
    esac
}

# List all series
sonarr_list() {
    _filter=""
    
    # Parse list-specific options
    while [ $# -gt 0 ]; do
        case "$1" in
            --monitored)
                _filter="monitored"
                shift
                ;;
            --unmonitored)
                _filter="unmonitored"
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
    
    # Get series
    _response="$(api_request GET "/api/v3/series")"
    
    # Apply filter
    case "$_filter" in
        monitored)
            _response="$(printf '%s' "$_response" | jq '[.[] | select(.monitored == true)]')"
            ;;
        unmonitored)
            _response="$(printf '%s' "$_response" | jq '[.[] | select(.monitored == false)]')"
            ;;
    esac
    
    # Format output
    printf '%s' "$_response" | format_table \
        "ID|Title|Year|Status|Monitored" \
        '.[] | [.id, .title, .year, .status, (if .monitored then "Yes" else "No" end)]'
}

# Search for series
sonarr_search() {
    _limit=10
    _term=""
    
    # Parse search-specific options
    while [ $# -gt 0 ]; do
        case "$1" in
            --limit)
                if [ -z "${2:-}" ]; then
                    die "--limit requires a number"
                fi
                _limit="$2"
                shift 2
                ;;
            --limit=*)
                _limit="${1#--limit=}"
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            -*)
                die "Unknown option: $1"
                ;;
            *)
                _term="$1"
                shift
                ;;
        esac
    done
    
    if [ -z "$_term" ]; then
        die "Search term required. Usage: arrctl sonarr search \"Show Name\""
    fi
    
    # URL encode the search term
    _encoded_term="$(url_encode "$_term")"
    
    # Search for series
    _response="$(api_request GET "/api/v3/series/lookup?term=${_encoded_term}")"
    
    # Apply limit
    _response="$(printf '%s' "$_response" | jq ".[:${_limit}]")"
    
    # Format output
    printf '%s' "$_response" | format_table \
        "TVDB ID|Title|Year|Network|Status" \
        '.[] | [.tvdbId, .title, .year, (.network // "N/A"), .status]'
}

# Add a series
sonarr_add() {
    _tvdb_id=""
    _quality=""
    _root=""
    _search=false
    _monitored=true
    
    # Parse add-specific options
    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                if [ -z "${2:-}" ]; then
                    die "--id requires a TVDB ID"
                fi
                _tvdb_id="$2"
                shift 2
                ;;
            --id=*)
                _tvdb_id="${1#--id=}"
                shift
                ;;
            --quality)
                if [ -z "${2:-}" ]; then
                    die "--quality requires a profile name or ID"
                fi
                _quality="$2"
                shift 2
                ;;
            --quality=*)
                _quality="${1#--quality=}"
                shift
                ;;
            --root)
                if [ -z "${2:-}" ]; then
                    die "--root requires a path"
                fi
                _root="$2"
                shift 2
                ;;
            --root=*)
                _root="${1#--root=}"
                shift
                ;;
            --search)
                _search=true
                shift
                ;;
            --monitored)
                _monitored=true
                shift
                ;;
            --no-monitored)
                _monitored=false
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
    
    if [ -z "$_tvdb_id" ]; then
        die "TVDB ID required. Usage: arrctl sonarr add --id <tvdb_id>"
    fi
    
    # Lookup series by TVDB ID to get full metadata
    [ "$QUIET_MODE" -eq 0 ] && info "Looking up series with TVDB ID: $_tvdb_id"
    _series="$(api_request GET "/api/v3/series/lookup?term=tvdb:${_tvdb_id}")"
    
    # Check if we got results
    _count="$(printf '%s' "$_series" | jq 'length')"
    if [ "$_count" -eq 0 ]; then
        die "No series found with TVDB ID: $_tvdb_id"
    fi
    
    # Get first result
    _series="$(printf '%s' "$_series" | jq '.[0]')"
    _title="$(printf '%s' "$_series" | jq -r '.title')"
    
    [ "$QUIET_MODE" -eq 0 ] && info "Found: $_title"
    
    # Resolve quality profile
    _quality_id="$(_sonarr_resolve_quality_profile "$_quality")"
    [ "$QUIET_MODE" -eq 0 ] && info "Using quality profile ID: $_quality_id"
    
    # Resolve root folder
    _root_path="$(_sonarr_resolve_root_folder "$_root")"
    [ "$QUIET_MODE" -eq 0 ] && info "Using root folder: $_root_path"
    
    # Build the POST payload
    _payload="$(printf '%s' "$_series" | jq \
        --argjson qid "$_quality_id" \
        --arg root "$_root_path" \
        --argjson monitored "$_monitored" \
        --argjson search "$_search" \
        '{
            title: .title,
            tvdbId: .tvdbId,
            qualityProfileId: $qid,
            rootFolderPath: $root,
            monitored: $monitored,
            seasonFolder: true,
            addOptions: {
                searchForMissingEpisodes: $search
            },
            images: .images,
            seasons: .seasons
        }'
    )"
    
    # Add the series
    _result="$(api_request POST "/api/v3/series" "$_payload")"
    
    # Output result
    if [ "$QUIET_MODE" -eq 0 ]; then
        _id="$(printf '%s' "$_result" | jq -r '.id')"
        info "Successfully added: $_title (ID: $_id)"
    fi
    
    # Output JSON if not in table mode
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        printf '%s\n' "$_result" | jq '.'
    fi
}


# Show detailed series information
sonarr_info() {
    _id=""
    _name=""

    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                [ -n "${2:-}" ] || die "--id requires a series ID"
                _id="$2"
                shift 2
                ;;
            --id=*)
                _id="${1#--id=}"
                shift
                ;;
            --name)
                [ -n "${2:-}" ] || die "--name requires a value"
                _name="$2"
                shift 2
                ;;
            --name=*)
                _name="${1#--name=}"
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done

    if [ -n "$_id" ] && [ -n "$_name" ]; then
        die "Use either --id or --name, not both"
    fi
    if [ -z "$_id" ] && [ -z "$_name" ]; then
        die "Either --id or --name is required"
    fi

    _series_all="$(api_request GET "/api/v3/series")"

    if [ -n "$_id" ]; then
        _series="$(printf '%s' "$_series_all" | jq --argjson id "$_id" '[.[] | select(.id == $id)]')"
    else
        _series="$(printf '%s' "$_series_all" | jq --arg name "$_name" '[.[] | select((.title // "") | ascii_downcase | contains($name | ascii_downcase))]')"
    fi

    _count="$(printf '%s' "$_series" | jq 'length')"
    [ "$_count" -gt 0 ] || die "No matching series found"

    _profiles="$(api_request GET "/api/v3/qualityprofile")"
    _profile_lookup="$(printf '%s' "$_profiles" | jq 'map({(.id|tostring): .name}) | add // {}')"
    _tags="$(api_request GET "/api/v3/tag")"
    _tag_lookup="$(printf '%s' "$_tags" | jq 'map({(.id|tostring): .label}) | add // {}')"

    _out='[]'
    _idx=0
    while [ "$_idx" -lt "$_count" ]; do
        _one="$(printf '%s' "$_series" | jq ".[${_idx}]")"
        _series_id="$(printf '%s' "$_one" | jq -r '.id')"

        _episodes="$(api_request GET "/api/v3/episode?seriesId=${_series_id}")"
        _episode_count="$(printf '%s' "$_episodes" | jq 'length')"
        _episode_files="$(api_request GET "/api/v3/episodefile?seriesId=${_series_id}")"

        _enriched="$(printf '%s' "$_one" | jq \
            --argjson profiles "$_profile_lookup" \
            --argjson tags "$_tag_lookup" \
            --argjson episodeCount "$_episode_count" \
            --argjson episodeFiles "$_episode_files" \
            '{
                id,
                title,
                year,
                status,
                monitored,
                qualityProfileName: ($profiles[(.qualityProfileId|tostring)] // "Unknown"),
                rootFolder: (.rootFolderPath // ""),
                overview: (.overview // ""),
                tags: ((.tags // []) | map($tags[(tostring)] // ("Tag-" + (tostring)))),
                seasonsCount: ((.seasons // []) | length),
                episodesCount: $episodeCount,
                episodeFiles: ($episodeFiles | map(.relativePath // .path // ("ID:" + (.id|tostring))))
            }')"

        _out="$(printf '%s\n%s' "$_out" "$_enriched" | jq -s '.[0] + [.[1]]')"
        _idx=$((_idx + 1))
    done

    printf '%s' "$_out" | format_table \
        "ID|Title|Year|Status|Monitored|Quality Profile|Root Folder|Seasons|Episodes|Tags|Episode Files|Overview" \
        '.[] | [
            .id,
            .title,
            (.year // ""),
            .status,
            (if .monitored then "Yes" else "No" end),
            .qualityProfileName,
            .rootFolder,
            .seasonsCount,
            .episodesCount,
            ((.tags // []) | join(", ")),
            ((.episodeFiles // []) | join(", ")),
            .overview
        ]'
}

# Delete series by ID
sonarr_delete() {
    _id=""
    _delete_files=false
    _add_exclusion=false
    _yes=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                [ -n "${2:-}" ] || die "--id requires a series ID"
                _id="$2"
                shift 2
                ;;
            --id=*)
                _id="${1#--id=}"
                shift
                ;;
            --delete-files)
                _delete_files=true
                shift
                ;;
            --add-exclusion)
                _add_exclusion=true
                shift
                ;;
            --yes)
                _yes=true
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done

    [ -n "$_id" ] || die "--id is required. Usage: arrctl sonarr delete --id <id>"

    _series="$(api_request GET "/api/v3/series/${_id}")"
    _title="$(printf '%s' "$_series" | jq -r '.title // "Unknown"')"

    if [ "$_yes" != "true" ]; then
        printf 'Delete Sonarr series "%s" (ID: %s)? [y/N]: ' "$_title" "$_id" >&2
        if [ -t 0 ]; then
            read -r _confirm
        else
            read -r _confirm </dev/tty
        fi
        case "$_confirm" in
            y|Y|yes|YES) ;;
            *)
                info "Cancelled"
                return 0
                ;;
        esac
    fi

    _endpoint="/api/v3/series/${_id}?deleteFiles=${_delete_files}&addImportListExclusion=${_add_exclusion}"
    api_request DELETE "$_endpoint" >/dev/null

    if [ "$OUTPUT_FORMAT" = "json" ]; then
        jq -n \
            --arg service "sonarr" \
            --argjson id "$_id" \
            --arg title "$_title" \
            --argjson deleteFiles "$_delete_files" \
            --argjson addExclusion "$_add_exclusion" \
            '{service: $service, deleted: true, id: $id, title: $title, deleteFiles: $deleteFiles, addImportListExclusion: $addExclusion}'
    else
        info "Deleted Sonarr series: $_title (ID: $_id)"
    fi
}

# Resolve quality profile - by ID, name, config default, or first available
# Usage: _sonarr_resolve_quality_profile "profile_name_or_id"
_sonarr_resolve_quality_profile() {
    _input="${1:-}"
    
    # Get all quality profiles
    _profiles="$(api_request GET "/api/v3/qualityprofile")"
    
    # If input provided, try to match it
    if [ -n "$_input" ]; then
        # Try as ID first (numeric)
        case "$_input" in
            *[!0-9]*)
                # Not numeric, try as name
                _id="$(printf '%s' "$_profiles" | jq -r --arg name "$_input" \
                    '.[] | select(.name == $name) | .id // empty' | head -n1)"
                ;;
            *)
                # Numeric, try as ID
                _id="$(printf '%s' "$_profiles" | jq -r --argjson id "$_input" \
                    '.[] | select(.id == $id) | .id // empty' | head -n1)"
                ;;
        esac
        
        if [ -n "$_id" ]; then
            printf '%s' "$_id"
            return 0
        fi
        
        die "Quality profile not found: $_input"
    fi
    
    # No input - check config default
    _default="$(get_config_value "sonarr" "defaults.qualityProfile" "")"
    if [ -n "$_default" ]; then
        # Try to match default by name
        _id="$(printf '%s' "$_profiles" | jq -r --arg name "$_default" \
            '.[] | select(.name == $name) | .id // empty' | head -n1)"
        if [ -n "$_id" ]; then
            printf '%s' "$_id"
            return 0
        fi
        warn "Configured default quality profile '$_default' not found, using first available"
    fi
    
    # Fall back to first available
    _id="$(printf '%s' "$_profiles" | jq -r '.[0].id // empty')"
    if [ -z "$_id" ]; then
        die "No quality profiles available in Sonarr"
    fi
    
    printf '%s' "$_id"
}

# Resolve root folder - by path, config default, or first available
# Usage: _sonarr_resolve_root_folder "path"
_sonarr_resolve_root_folder() {
    _input="${1:-}"
    
    # Get all root folders
    _folders="$(api_request GET "/api/v3/rootfolder")"
    
    # If input provided, try to match it
    if [ -n "$_input" ]; then
        _path="$(printf '%s' "$_folders" | jq -r --arg path "$_input" \
            '.[] | select(.path == $path) | .path // empty' | head -n1)"
        
        if [ -n "$_path" ]; then
            printf '%s' "$_path"
            return 0
        fi
        
        die "Root folder not found: $_input"
    fi
    
    # No input - check config default
    _default="$(get_config_value "sonarr" "defaults.rootFolder" "")"
    if [ -n "$_default" ]; then
        # Try to match default by path
        _path="$(printf '%s' "$_folders" | jq -r --arg path "$_default" \
            '.[] | select(.path == $path) | .path // empty' | head -n1)"
        if [ -n "$_path" ]; then
            printf '%s' "$_path"
            return 0
        fi
        warn "Configured default root folder '$_default' not found, using first available"
    fi
    
    # Fall back to first available
    _path="$(printf '%s' "$_folders" | jq -r '.[0].path // empty')"
    if [ -z "$_path" ]; then
        die "No root folders configured in Sonarr"
    fi
    
    printf '%s' "$_path"
}

# Get calendar entries (upcoming episodes)
# Usage: sonarr_calendar [--days N] [--start DATE] [--end DATE]
sonarr_calendar() {
    _days=7
    _start=""
    _end=""
    
    # Parse calendar-specific options
    while [ $# -gt 0 ]; do
        case "$1" in
            --days)
                if [ -z "${2:-}" ]; then
                    die "--days requires a number"
                fi
                _days="$2"
                shift 2
                ;;
            --days=*)
                _days="${1#--days=}"
                shift
                ;;
            --start)
                if [ -z "${2:-}" ]; then
                    die "--start requires a date (YYYY-MM-DD)"
                fi
                _start="$2"
                shift 2
                ;;
            --start=*)
                _start="${1#--start=}"
                shift
                ;;
            --end)
                if [ -z "${2:-}" ]; then
                    die "--end requires a date (YYYY-MM-DD)"
                fi
                _end="$2"
                shift 2
                ;;
            --end=*)
                _end="${1#--end=}"
                shift
                ;;
            -h|--help)
                sonarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
    
    # Calculate dates if not provided
    if [ -z "$_start" ]; then
        _start=$(date +%Y-%m-%d)
    fi
    if [ -z "$_end" ]; then
        # Try BSD date first, fall back to GNU date
        _end=$(date -v+"${_days}"d +%Y-%m-%d 2>/dev/null || date -d "+${_days} days" +%Y-%m-%d)
    fi
    
    # Fetch all series for title lookup (in-memory cache)
    _series_list="$(api_request GET "/api/v3/series")"
    _series_lookup="$(printf '%s' "$_series_list" | jq 'map({(.id | tostring): .title}) | add // {}')"
    
    # Fetch calendar data
    _response="$(api_request GET "/api/v3/calendar?start=${_start}&end=${_end}")"
    
    # Format output - transform to unified format with date, title, episode, service
    printf '%s' "$_response" | jq --argjson lookup "$_series_lookup" '[.[] | {
        date: (.airDateUtc | split("T")[0]),
        title: ($lookup[(.seriesId | tostring)] // "Unknown Series"),
        episode: ("S" + (if .seasonNumber < 10 then "0" else "" end) + (.seasonNumber | tostring) + "E" + (if .episodeNumber < 10 then "0" else "" end) + (.episodeNumber | tostring) + " - " + .title),
        service: "Sonarr"
    }]'
}

# Get calendar entries in raw JSON format for merging
# Usage: sonarr_calendar_raw start_date end_date
# Outputs normalized JSON array for calendar merging
sonarr_calendar_raw() {
    _start="$1"
    _end="$2"
    
    # Fetch all series for title lookup (in-memory cache)
    _series_list="$(api_request GET "/api/v3/series")"
    _series_lookup="$(printf '%s' "$_series_list" | jq 'map({(.id | tostring): .title}) | add // {}')"
    
    # Fetch calendar data
    _response="$(api_request GET "/api/v3/calendar?start=${_start}&end=${_end}")"
    
    # Transform to unified format with series title lookup
    printf '%s' "$_response" | jq --argjson lookup "$_series_lookup" '[.[] | {
        date: (.airDateUtc | split("T")[0]),
        title: ($lookup[(.seriesId | tostring)] // "Unknown Series"),
        episode: ("S" + (if .seasonNumber < 10 then "0" else "" end) + (.seasonNumber | tostring) + "E" + (if .episodeNumber < 10 then "0" else "" end) + (.episodeNumber | tostring) + " - " + .title),
        service: "Sonarr"
    }]'
}
