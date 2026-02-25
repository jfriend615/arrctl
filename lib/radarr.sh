#!/bin/sh
# radarr.sh - Radarr service commands for arrctl
# POSIX compliant - tested with dash

# Show help for radarr commands
radarr_help() {
    cat <<EOF
arrctl radarr - Manage Radarr (Movies)

Usage: arrctl radarr <command> [options]

Commands:
    list              List all movies in library
    search <term>     Search for movies by name
    add               Add a movie to library
    info              Show detailed movie information
    delete            Delete a movie from library
    calendar          Show upcoming movie releases

List Options:
    --monitored       Show only monitored movies
    --unmonitored     Show only unmonitored movies
    --format FORMAT   Output format: json|table|auto (default: auto)
    -q, --quiet       Suppress non-essential output

Search Options:
    --limit N         Limit results (default: 10)
    --format FORMAT   Output format: json|table|auto (default: auto)

Add Options:
    --id TMDB_ID      TMDb ID of movie to add (required)
    --quality NAME    Quality profile name or ID
    --root PATH       Root folder path
    --search          Start search for movie after adding
    --monitored       Monitor the movie (default: true)
    --no-monitored    Don't monitor the movie

Info Options:
    --id ID           Radarr movie ID (exact match)
    --name NAME       Movie name (partial match)
    --format FORMAT   Output format: json|table|auto (default: auto)

Delete Options:
    --id ID           Radarr movie ID to delete (required)
    --delete-files    Also delete movie files from disk
    --add-exclusion   Add import list exclusion for this movie
    --yes             Skip confirmation prompt

Calendar Options:
    --days N          Show next N days (default: 7)
    --start DATE      Start date (YYYY-MM-DD)
    --end DATE        End date (YYYY-MM-DD)
    --format FORMAT   Output format: json|table|auto (default: auto)

Examples:
    arrctl radarr list
    arrctl radarr list --monitored --format table
    arrctl radarr search "The Matrix"
    arrctl radarr search "Dune" --limit 5
    arrctl radarr info --name "Dune"
    arrctl radarr info --id 100 --format json
    arrctl radarr add --id 603 --quality "HD-1080p" --search
    arrctl radarr delete --id 100 --delete-files
    arrctl radarr calendar --days 30

EOF
}

# Main entry point for radarr commands
radarr_main() {
    # Handle no arguments or help before loading config
    if [ $# -eq 0 ]; then
        radarr_help
        return 0
    fi
    
    # Check for help flags first (before config load)
    for _arg in "$@"; do
        case "$_arg" in
            -h|--help|help)
                radarr_help
                return 0
                ;;
        esac
    done
    
    # Load configuration (only needed for actual commands)
    load_config "radarr"
    
    # Parse service-level args
    parse_service_args "$@"
    eval "set -- $_REMAINING_ARGS"
    
    case "$1" in
        list)
            shift
            radarr_list "$@"
            ;;
        search)
            shift
            radarr_search "$@"
            ;;
        add)
            shift
            radarr_add "$@"
            ;;
        info)
            shift
            radarr_info "$@"
            ;;
        delete)
            shift
            radarr_delete "$@"
            ;;
        calendar)
            shift
            radarr_calendar "$@"
            ;;
        *)
            die "Unknown radarr command: $1. Use 'arrctl radarr --help' for usage."
            ;;
    esac
}

# List all movies
radarr_list() {
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
                radarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
    
    # Get movies
    _response="$(api_request GET "/api/v3/movie")"
    
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

# Search for movies
radarr_search() {
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
                radarr_help
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
        die "Search term required. Usage: arrctl radarr search \"Movie Name\""
    fi
    
    # URL encode the search term
    _encoded_term="$(url_encode "$_term")"
    
    # Search for movies
    _response="$(api_request GET "/api/v3/movie/lookup?term=${_encoded_term}")"
    
    # Apply limit
    _response="$(printf '%s' "$_response" | jq ".[:${_limit}]")"
    
    # Format output
    printf '%s' "$_response" | format_table \
        "TMDb ID|Title|Year|Status" \
        '.[] | [.tmdbId, .title, .year, .status]'
}

# Add a movie
radarr_add() {
    _tmdb_id=""
    _quality=""
    _root=""
    _search=false
    _monitored=true
    
    # Parse add-specific options
    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                if [ -z "${2:-}" ]; then
                    die "--id requires a TMDb ID"
                fi
                _tmdb_id="$2"
                shift 2
                ;;
            --id=*)
                _tmdb_id="${1#--id=}"
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
                radarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done
    
    if [ -z "$_tmdb_id" ]; then
        die "TMDb ID required. Usage: arrctl radarr add --id <tmdb_id>"
    fi
    
    # Lookup movie by TMDb ID to get full metadata
    [ "$QUIET_MODE" -eq 0 ] && info "Looking up movie with TMDb ID: $_tmdb_id"
    _movie="$(api_request GET "/api/v3/movie/lookup?term=tmdb:${_tmdb_id}")"
    
    # Check if we got results
    _count="$(printf '%s' "$_movie" | jq 'length')"
    if [ "$_count" -eq 0 ]; then
        die "No movie found with TMDb ID: $_tmdb_id"
    fi
    
    # Get first result
    _movie="$(printf '%s' "$_movie" | jq '.[0]')"
    _title="$(printf '%s' "$_movie" | jq -r '.title')"
    
    [ "$QUIET_MODE" -eq 0 ] && info "Found: $_title"
    
    # Resolve quality profile
    _quality_id="$(_radarr_resolve_quality_profile "$_quality")"
    [ "$QUIET_MODE" -eq 0 ] && info "Using quality profile ID: $_quality_id"
    
    # Resolve root folder
    _root_path="$(_radarr_resolve_root_folder "$_root")"
    [ "$QUIET_MODE" -eq 0 ] && info "Using root folder: $_root_path"
    
    # Build the POST payload (simpler than Sonarr - no seasons/seasonFolder)
    _payload="$(printf '%s' "$_movie" | jq \
        --argjson qid "$_quality_id" \
        --arg root "$_root_path" \
        --argjson monitored "$_monitored" \
        --argjson search "$_search" \
        '{
            title: .title,
            tmdbId: .tmdbId,
            qualityProfileId: $qid,
            rootFolderPath: $root,
            monitored: $monitored,
            addOptions: {
                searchForMovie: $search
            },
            images: .images
        }'
    )"
    
    # Add the movie
    _result="$(api_request POST "/api/v3/movie" "$_payload")"
    
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


# Show detailed movie information
radarr_info() {
    _id=""
    _name=""

    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                [ -n "${2:-}" ] || die "--id requires a movie ID"
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
                radarr_help
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

    if [ -n "$_id" ]; then
        case "$_id" in
            *[!0-9]*|'') die "--id must be a numeric Radarr movie ID" ;;
        esac
    fi

    _movies_all="$(api_request GET "/api/v3/movie")"

    if [ -n "$_id" ]; then
        _movies="$(printf '%s' "$_movies_all" | jq --argjson id "$_id" '[.[] | select(.id == $id)]')"
    else
        _movies="$(printf '%s' "$_movies_all" | jq --arg name "$_name" '[.[] | select((.title // "") | ascii_downcase | contains($name | ascii_downcase))]')"
    fi

    _count="$(printf '%s' "$_movies" | jq 'length')"
    [ "$_count" -gt 0 ] || die "No matching movie found"

    _profiles="$(api_request GET "/api/v3/qualityprofile")"
    _profile_lookup="$(printf '%s' "$_profiles" | jq 'map({(.id|tostring): .name}) | add // {}')"
    _tags="$(api_request GET "/api/v3/tag")"
    _tag_lookup="$(printf '%s' "$_tags" | jq 'map({(.id|tostring): .label}) | add // {}')"

    _out='[]'
    _idx=0
    while [ "$_idx" -lt "$_count" ]; do
        _one="$(printf '%s' "$_movies" | jq ".[${_idx}]")"

        _movie_file_id="$(printf '%s' "$_one" | jq -r '.movieFile.id // .movieFileId // empty')"
        _movie_file='null'
        if [ -n "$_movie_file_id" ]; then
            _movie_file="$(api_request GET "/api/v3/moviefile/${_movie_file_id}")"
        fi

        _enriched="$(printf '%s' "$_one" | jq \
            --argjson profiles "$_profile_lookup" \
            --argjson tags "$_tag_lookup" \
            --argjson movieFile "$_movie_file" \
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
                movieFile: ($movieFile // .movieFile // null)
            }')"

        _out="$(printf '%s\n%s' "$_out" "$_enriched" | jq -s '.[0] + [.[1]]')"
        _idx=$((_idx + 1))
    done

    printf '%s' "$_out" | format_table \
        "ID|Title|Year|Status|Monitored|Quality Profile|Root Folder|Tags|Movie File|Overview" \
        '.[] | [
            .id,
            .title,
            (.year // ""),
            .status,
            (if .monitored then "Yes" else "No" end),
            .qualityProfileName,
            .rootFolder,
            ((.tags // []) | join(", ")),
            (if (.movieFile // null) == null then "Not Downloaded" else ((.movieFile.relativePath // .movieFile.path // "") + " | " + (((.movieFile.size // 0) / (1024*1024*1024) | floor | tostring) + " GB") + " | " + (.movieFile.quality.quality.name // "Unknown")) end),
            .overview
        ]'
}

# Delete movie by ID
radarr_delete() {
    _id=""
    _delete_files=false
    _add_exclusion=false
    _yes=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --id)
                [ -n "${2:-}" ] || die "--id requires a movie ID"
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
                radarr_help
                return 0
                ;;
            *)
                die "Unknown option: $1"
                ;;
        esac
    done

    [ -n "$_id" ] || die "--id is required. Usage: arrctl radarr delete --id <id>"
    case "$_id" in
        *[!0-9]*|'') die "--id must be a numeric Radarr movie ID" ;;
    esac

    _movie="$(api_request GET "/api/v3/movie/${_id}")"
    _title="$(printf '%s' "$_movie" | jq -r '.title // "Unknown"')"

    if [ "$_yes" != "true" ]; then
        printf 'Delete Radarr movie "%s" (ID: %s)? [y/N]: ' "$_title" "$_id" >&2
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

    _endpoint="/api/v3/movie/${_id}?deleteFiles=${_delete_files}&addImportListExclusion=${_add_exclusion}"
    api_request DELETE "$_endpoint" >/dev/null

    if [ "$OUTPUT_FORMAT" = "json" ]; then
        jq -n \
            --arg service "radarr" \
            --argjson id "$_id" \
            --arg title "$_title" \
            --argjson deleteFiles "$_delete_files" \
            --argjson addExclusion "$_add_exclusion" \
            '{service: $service, deleted: true, id: $id, title: $title, deleteFiles: $deleteFiles, addImportListExclusion: $addExclusion}'
    else
        info "Deleted Radarr movie: $_title (ID: $_id)"
    fi
}

# Resolve quality profile - by ID, name, config default, or first available
# Usage: _radarr_resolve_quality_profile "profile_name_or_id"
_radarr_resolve_quality_profile() {
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
    _default="$(get_config_value "radarr" "defaults.qualityProfile" "")"
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
        die "No quality profiles available in Radarr"
    fi
    
    printf '%s' "$_id"
}

# Resolve root folder - by path, config default, or first available
# Usage: _radarr_resolve_root_folder "path"
_radarr_resolve_root_folder() {
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
    _default="$(get_config_value "radarr" "defaults.rootFolder" "")"
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
        die "No root folders configured in Radarr"
    fi
    
    printf '%s' "$_path"
}

# Get calendar entries (upcoming movie releases)
# Usage: radarr_calendar [--days N] [--start DATE] [--end DATE]
radarr_calendar() {
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
                radarr_help
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
    
    # Fetch calendar data
    _response="$(api_request GET "/api/v3/calendar?start=${_start}&end=${_end}")"
    
    # Format output - transform to unified format with date, title, episode (empty for movies), service
    # Use digitalRelease if available, otherwise inCinemas, otherwise physicalRelease
    printf '%s' "$_response" | jq '[.[] | {
        date: ((.digitalRelease // .inCinemas // .physicalRelease // "") | split("T")[0]),
        title: .title,
        episode: "",
        service: "Radarr"
    }] | map(select(.date != ""))'
}

# Get calendar entries in raw JSON format for merging
# Usage: radarr_calendar_raw start_date end_date
# Outputs normalized JSON array for calendar merging
radarr_calendar_raw() {
    _start="$1"
    _end="$2"
    
    # Fetch calendar data
    _response="$(api_request GET "/api/v3/calendar?start=${_start}&end=${_end}")"
    
    # Transform to unified format
    printf '%s' "$_response" | jq '[.[] | {
        date: ((.digitalRelease // .inCinemas // .physicalRelease // "") | split("T")[0]),
        title: .title,
        episode: "",
        service: "Radarr"
    }] | map(select(.date != ""))'
}
