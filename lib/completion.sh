#!/bin/sh
# completion.sh - Shell completion management for arrctl
# POSIX compliant

completion_help() {
    cat <<EOF
arrctl completion - Shell completion support

Usage: arrctl completion [options]

Options:
    --install         Install completion for current shell profile
    --shell SHELL     Shell type: bash|zsh (auto-detect by default)
    -h, --help        Show this help message

Examples:
    arrctl completion --install
    arrctl completion --install --shell bash
    arrctl completion --shell zsh
EOF
}

_detect_shell() {
    if [ -n "${1:-}" ]; then
        printf '%s' "$1"
        return
    fi

    _shell_name="$(basename "${SHELL:-}")"
    case "$_shell_name" in
        bash|zsh) printf '%s' "$_shell_name" ;;
        *) die "Could not detect shell. Use --shell bash|zsh" ;;
    esac
}

_completion_file_for_shell() {
    case "$1" in
        bash) printf '%s/completions/arrctl.bash' "$INSTALL_DIR" ;;
        zsh) printf '%s/completions/_arrctl' "$INSTALL_DIR" ;;
        *) die "Unsupported shell: $1 (supported: bash, zsh)" ;;
    esac
}

_profile_for_shell() {
    case "$1" in
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                printf '%s/.bashrc' "$HOME"
            else
                printf '%s/.bash_profile' "$HOME"
            fi
            ;;
        zsh)
            printf '%s/.zshrc' "$HOME"
            ;;
        *)
            die "Unsupported shell: $1"
            ;;
    esac
}

_profile_block_for_shell() {
    _shell="$1"
    _completion_file="$2"

    case "$_shell" in
        bash)
            cat <<EOF
# >>> arrctl completion >>>
if [ -f "$_completion_file" ]; then
    . "$_completion_file"
fi
# <<< arrctl completion <<<
EOF
            ;;
        zsh)
            cat <<EOF
# >>> arrctl completion >>>
if [ -f "$_completion_file" ]; then
    source "$_completion_file"
fi
# <<< arrctl completion <<<
EOF
            ;;
    esac
}

_install_profile_block() {
    _profile="$1"
    _new_block="$2"

    [ -f "$_profile" ] || : > "$_profile"

    _tmp="$(mktemp)"
    awk '
        BEGIN { skip=0 }
        /^# >>> arrctl completion >>>$/ { skip=1; next }
        /^# <<< arrctl completion <<</ { skip=0; next }
        skip==0 { print }
    ' "$_profile" > "$_tmp"

    {
        cat "$_tmp"
        printf '\n%s\n' "$_new_block"
    } > "$_profile"

    rm -f "$_tmp"
}

completion_install() {
    _shell="$(_detect_shell "${1:-}")"
    _completion_file="$(_completion_file_for_shell "$_shell")"
    _profile="$(_profile_for_shell "$_shell")"

    [ -f "$_completion_file" ] || die "Completion file not found: $_completion_file"

    _block="$(_profile_block_for_shell "$_shell" "$_completion_file")"
    _install_profile_block "$_profile" "$_block"

    info "Installed $_shell completion in $_profile"
    info "Run: source $_profile"
}

completion_main() {
    _install=0
    _shell=""

    while [ $# -gt 0 ]; do
        case "$1" in
            --install)
                _install=1
                shift
                ;;
            --shell)
                [ -n "${2:-}" ] || die "--shell requires a value (bash|zsh)"
                _shell="$2"
                shift 2
                ;;
            --shell=*)
                _shell="${1#--shell=}"
                shift
                ;;
            -h|--help|help)
                completion_help
                return 0
                ;;
            *)
                die "Unknown option: $1. Use 'arrctl completion --help' for usage."
                ;;
        esac
    done

    if [ "$_install" -eq 1 ]; then
        completion_install "$_shell"
        return 0
    fi

    _shell="$(_detect_shell "$_shell")"
    _completion_file="$(_completion_file_for_shell "$_shell")"
    cat "$_completion_file"
}
