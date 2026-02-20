#!/bin/sh
# Install arrctl shell completions (idempotent)
# Usage: ./completions/install.sh [--shell bash|zsh]

set -e

resolve_link() {
    TARGET="$1"
    cd "$(dirname "$TARGET")"
    TARGET=$(basename "$TARGET")
    while [ -L "$TARGET" ]; do
        TARGET=$(readlink "$TARGET")
        cd "$(dirname "$TARGET")"
        TARGET=$(basename "$TARGET")
    done
    pwd -P
}

SCRIPT_DIR="$(resolve_link "$0")"
INSTALL_DIR="$(cd "$SCRIPT_DIR/.." && pwd -P)"
LIB_DIR="$INSTALL_DIR/lib"

# shellcheck source=lib/common.sh
# shellcheck disable=SC1091
. "$LIB_DIR/common.sh"
# shellcheck source=lib/completion.sh
# shellcheck disable=SC1091
. "$LIB_DIR/completion.sh"

_shell=""
while [ $# -gt 0 ]; do
    case "$1" in
        --shell)
            [ -n "${2:-}" ] || die "--shell requires bash|zsh"
            _shell="$2"
            shift 2
            ;;
        --shell=*)
            _shell="${1#--shell=}"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--shell bash|zsh]"
            exit 0
            ;;
        *)
            die "Unknown option: $1"
            ;;
    esac
done

completion_install "$_shell"
