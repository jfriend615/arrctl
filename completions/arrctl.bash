# bash completion for arrctl

_arrctl_completions() {
    local cur prev cmd subcmd

    _arrctl_set_reply() {
        COMPREPLY=()
        while IFS= read -r candidate; do
            COMPREPLY+=("$candidate")
        done < <(compgen -W "$1" -- "$cur")
    }

    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Global options that take a value
    case "$prev" in
        --config|--format|--shell|--days|--limit|--start|--end|--id|--quality|--root|--message|--reason|--library|--min-days|--max-plays|--min-size-gb)
            return 0
            ;;
    esac

    cmd="${COMP_WORDS[1]}"
    subcmd="${COMP_WORDS[2]}"

    if [[ $COMP_CWORD -eq 1 ]]; then
        _arrctl_set_reply "sonarr radarr overseerr tautulli calendar completion update -h --help -v --version --config"
        return 0
    fi

    case "$cmd" in
        sonarr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                _arrctl_set_reply "list search add calendar help -h --help --config --format -q --quiet"
                return 0
            fi
            case "$subcmd" in
                list)
                    _arrctl_set_reply "--monitored --unmonitored --format -q --quiet -h --help --config"
                    ;;
                search)
                    _arrctl_set_reply "--limit --format -h --help --config"
                    ;;
                add)
                    _arrctl_set_reply "--id --quality --root --search --monitored --no-monitored --format -q --quiet -h --help --config"
                    ;;
                calendar)
                    _arrctl_set_reply "--days --start --end --format -h --help --config"
                    ;;
            esac
            ;;
        radarr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                _arrctl_set_reply "list search add calendar help -h --help --config --format -q --quiet"
                return 0
            fi
            case "$subcmd" in
                list)
                    _arrctl_set_reply "--monitored --unmonitored --format -q --quiet -h --help --config"
                    ;;
                search)
                    _arrctl_set_reply "--limit --format -h --help --config"
                    ;;
                add)
                    _arrctl_set_reply "--id --quality --root --search --monitored --no-monitored --format -q --quiet -h --help --config"
                    ;;
                calendar)
                    _arrctl_set_reply "--days --start --end --format -h --help --config"
                    ;;
            esac
            ;;
        overseerr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                _arrctl_set_reply "pending approve deny decline help -h --help --config --format"
                return 0
            fi
            case "$subcmd" in
                pending)
                    _arrctl_set_reply "--format -h --help --config"
                    ;;
                approve)
                    _arrctl_set_reply "--message --format -h --help --config"
                    ;;
                deny|decline)
                    _arrctl_set_reply "--reason --format -h --help --config"
                    ;;
            esac
            ;;
        tautulli)
            if [[ $COMP_CWORD -eq 2 ]]; then
                _arrctl_set_reply "now stale help -h --help --config --format -q --quiet"
                return 0
            fi
            case "$subcmd" in
                now)
                    _arrctl_set_reply "--format -q --quiet -h --help --config"
                    ;;
                stale)
                    _arrctl_set_reply "--library --min-days --max-plays --min-size-gb --limit --json --format -h --help --config"
                    ;;
            esac
            ;;
        calendar)
            _arrctl_set_reply "--days --week --sonarr --radarr --format -h --help --config"
            ;;
        completion)
            _arrctl_set_reply "--install --shell bash zsh -h --help"
            ;;
        update)
            COMPREPLY=()
            ;;
    esac

    return 0
}

complete -F _arrctl_completions arrctl
