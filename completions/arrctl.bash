# bash completion for arrctl

_arrctl_completions() {
    local cur prev words cword cmd subcmd
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
        COMPREPLY=( $(compgen -W "sonarr radarr overseerr tautulli calendar completion update -h --help -v --version --config" -- "$cur") )
        return 0
    fi

    case "$cmd" in
        sonarr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "list search add calendar help -h --help --config --format -q --quiet" -- "$cur") )
                return 0
            fi
            case "$subcmd" in
                list)
                    COMPREPLY=( $(compgen -W "--monitored --unmonitored --format -q --quiet -h --help --config" -- "$cur") )
                    ;;
                search)
                    COMPREPLY=( $(compgen -W "--limit --format -h --help --config" -- "$cur") )
                    ;;
                add)
                    COMPREPLY=( $(compgen -W "--id --quality --root --search --monitored --no-monitored --format -q --quiet -h --help --config" -- "$cur") )
                    ;;
                calendar)
                    COMPREPLY=( $(compgen -W "--days --start --end --format -h --help --config" -- "$cur") )
                    ;;
            esac
            ;;
        radarr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "list search add calendar help -h --help --config --format -q --quiet" -- "$cur") )
                return 0
            fi
            case "$subcmd" in
                list)
                    COMPREPLY=( $(compgen -W "--monitored --unmonitored --format -q --quiet -h --help --config" -- "$cur") )
                    ;;
                search)
                    COMPREPLY=( $(compgen -W "--limit --format -h --help --config" -- "$cur") )
                    ;;
                add)
                    COMPREPLY=( $(compgen -W "--id --quality --root --search --monitored --no-monitored --format -q --quiet -h --help --config" -- "$cur") )
                    ;;
                calendar)
                    COMPREPLY=( $(compgen -W "--days --start --end --format -h --help --config" -- "$cur") )
                    ;;
            esac
            ;;
        overseerr)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "pending approve deny decline help -h --help --config --format" -- "$cur") )
                return 0
            fi
            case "$subcmd" in
                pending)
                    COMPREPLY=( $(compgen -W "--format -h --help --config" -- "$cur") )
                    ;;
                approve)
                    COMPREPLY=( $(compgen -W "--message --format -h --help --config" -- "$cur") )
                    ;;
                deny|decline)
                    COMPREPLY=( $(compgen -W "--reason --format -h --help --config" -- "$cur") )
                    ;;
            esac
            ;;
        tautulli)
            if [[ $COMP_CWORD -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "now stale help -h --help --config --format -q --quiet" -- "$cur") )
                return 0
            fi
            case "$subcmd" in
                now)
                    COMPREPLY=( $(compgen -W "--format -q --quiet -h --help --config" -- "$cur") )
                    ;;
                stale)
                    COMPREPLY=( $(compgen -W "--library --min-days --max-plays --min-size-gb --limit --json --format -h --help --config" -- "$cur") )
                    ;;
            esac
            ;;
        calendar)
            COMPREPLY=( $(compgen -W "--days --week --sonarr --radarr --format -h --help --config" -- "$cur") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "--install --shell bash zsh -h --help" -- "$cur") )
            ;;
        update)
            COMPREPLY=()
            ;;
    esac

    return 0
}

complete -F _arrctl_completions arrctl
