#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ARRCTL="${ARRCTL_BIN:-${REPO_DIR}/bin/arrctl}"

pass() {
    printf 'PASS: %s\n' "$1"
}

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

test_case() {
    name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        pass "$name"
    else
        fail "$name"
    fi
}

test_case_fail_contains() {
    name="$1"
    needle="$2"
    shift 2
    tmp="$(mktemp)"
    if "$@" > /dev/null 2>"$tmp"; then
        rm -f "$tmp"
        fail "$name (expected failure)"
    fi
    if grep -q -- "$needle" "$tmp"; then
        rm -f "$tmp"
        pass "$name"
        return
    fi
    rm -f "$tmp"
    fail "$name (missing expected text: $needle)"
}

test_case "arrctl --help works" "$ARRCTL" --help
test_case "arrctl --version works" "$ARRCTL" --version
test_case "arrctl sonarr --help works" "$ARRCTL" sonarr --help
test_case "arrctl radarr --help works" "$ARRCTL" radarr --help
test_case "arrctl tautulli --help works" "$ARRCTL" tautulli --help
test_case "arrctl overseerr --help works" "$ARRCTL" overseerr --help
test_case "arrctl calendar --help works" "$ARRCTL" calendar --help
test_case "arrctl completion --help works" "$ARRCTL" completion --help

test_case_fail_contains "invalid format is rejected" "invalid format" "$ARRCTL" --format yaml sonarr list
test_case_fail_contains "conflicting monitored flags are rejected" "cannot be used together" "$ARRCTL" sonarr list --monitored --unmonitored

printf 'PASS: go smoke tests\n'
