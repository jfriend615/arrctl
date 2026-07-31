#!/bin/sh
# smoke.sh - CLI smoke tests for the Go arrctl binary

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
if [ -z "${ARRCTL_BIN:-}" ]; then
    printf '%s\n' "ARRCTL_BIN must point to a built Go arrctl binary; use 'make test-shell'" >&2
    exit 1
fi
ARRCTL="$ARRCTL_BIN"

# Colors (if terminal supports them)
RED=""
GREEN=""
YELLOW=""
NC=""
if [ -t 1 ]; then
    RED="$(printf '\033[0;31m')"
    GREEN="$(printf '\033[0;32m')"
    YELLOW="$(printf '\033[1;33m')"
    NC="$(printf '\033[0m')"
fi

# Test counters
PASSED=0
FAILED=0

# Test helper functions
pass() {
    PASSED=$((PASSED + 1))
    printf '%sPASS%s: %s\n' "$GREEN" "$NC" "$1"
}

fail() {
    FAILED=$((FAILED + 1))
    printf '%sFAIL%s: %s\n' "$RED" "$NC" "$1"
}

skip() {
    printf '%sSKIP%s: %s\n' "$YELLOW" "$NC" "$1"
}

# Run a test
test_case() {
    _name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        pass "$_name"
    else
        fail "$_name"
    fi
}

# Run a test expecting failure
test_case_fail() {
    _name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        fail "$_name (expected failure)"
    else
        pass "$_name"
    fi
}

# Run a test expecting failure with matching stderr text
test_case_fail_contains() {
    _name="$1"
    _needle="$2"
    shift 2

    _tmp="$(mktemp)"
    if "$@" > /dev/null 2>"$_tmp"; then
        rm -f "$_tmp"
        fail "$_name (expected failure)"
        return
    fi

    if grep -q -- "$_needle" "$_tmp"; then
        pass "$_name"
    else
        fail "$_name (missing expected error: $_needle)"
    fi
    rm -f "$_tmp"
}

printf '%s\n\n' "=== arrctl Smoke Tests ==="

# Validate the remaining shell-based installer and test tooling.
printf '%s\n' "--- Shell Tooling Checks ---"

if command -v shellcheck >/dev/null 2>&1; then
    test_case "shellcheck: installer and test tooling" shellcheck \
        "${REPO_DIR}/install.sh" \
        "${REPO_DIR}/test/smoke.sh" \
        "${REPO_DIR}/test/completion.sh" \
        "${REPO_DIR}/test/install-smoke.sh"
else
    skip "shellcheck not installed"
fi

if command -v dash >/dev/null 2>&1; then
    test_case "dash -n: install.sh" dash -n "${REPO_DIR}/install.sh"
    test_case "dash -n: test/smoke.sh" dash -n "${REPO_DIR}/test/smoke.sh"
    test_case "dash -n: test/install-smoke.sh" dash -n "${REPO_DIR}/test/install-smoke.sh"
else
    skip "dash not installed"
fi

test_case "bash -n: test/completion.sh" bash -n "${REPO_DIR}/test/completion.sh"

# Help output checks
printf '\n%s\n' "--- Help Commands ---"
test_case "arrctl --help works" "$ARRCTL" --help
test_case "arrctl --version works" "$ARRCTL" --version
skip "arrctl update test skipped (network-dependent)"
test_case "arrctl sonarr --help works" "$ARRCTL" sonarr --help
test_case "arrctl sonarr help works" "$ARRCTL" sonarr help
test_case "arrctl radarr --help works" "$ARRCTL" radarr --help
test_case "arrctl radarr help works" "$ARRCTL" radarr help
test_case "arrctl tautulli --help works" "$ARRCTL" tautulli --help
test_case "arrctl tautulli help works" "$ARRCTL" tautulli help
test_case "arrctl overseerr --help works" "$ARRCTL" overseerr --help
test_case "arrctl overseerr help works" "$ARRCTL" overseerr help
test_case "arrctl calendar --help works" "$ARRCTL" calendar --help
test_case "arrctl calendar help works" "$ARRCTL" calendar help
test_case "arrctl completion --help works" "$ARRCTL" completion --help
test_case "arrctl completion help works" "$ARRCTL" completion help

# Invalid command handling
printf '\n%s\n' "--- Error Handling ---"
test_case_fail "arrctl invalid-command fails" "$ARRCTL" invalid-command
test_case_fail "arrctl sonarr invalid-subcommand fails" "$ARRCTL" sonarr invalid-subcommand
test_case_fail "arrctl radarr invalid-subcommand fails" "$ARRCTL" radarr invalid-subcommand
test_case_fail "arrctl tautulli invalid-subcommand fails" "$ARRCTL" tautulli invalid-subcommand
test_case_fail "arrctl overseerr invalid-subcommand fails" "$ARRCTL" overseerr invalid-subcommand
test_case_fail_contains "invalid format is rejected" "invalid format" "$ARRCTL" --format yaml sonarr list
test_case_fail_contains "conflicting monitored flags are rejected" "cannot be used together" "$ARRCTL" sonarr list --monitored --unmonitored

# Help output contains expected content
printf '\n%s\n' "--- Help Content ---"
if "$ARRCTL" --help 2>&1 | grep -q "sonarr"; then
    pass "Main help mentions sonarr"
else
    fail "Main help mentions sonarr"
fi

if "$ARRCTL" --help 2>&1 | grep -q "radarr"; then
    pass "Main help mentions radarr"
else
    fail "Main help mentions radarr"
fi

if "$ARRCTL" --help 2>&1 | grep -q "tautulli"; then
    pass "Main help mentions tautulli"
else
    fail "Main help mentions tautulli"
fi

if "$ARRCTL" --help 2>&1 | grep -q "overseerr"; then
    pass "Main help mentions overseerr"
else
    fail "Main help mentions overseerr"
fi

if "$ARRCTL" --help 2>&1 | grep -q "update"; then
    pass "Main help mentions update"
else
    fail "Main help mentions update"
fi

if "$ARRCTL" --help 2>&1 | grep -q "calendar"; then
    pass "Main help mentions calendar"
else
    fail "Main help mentions calendar"
fi

if "$ARRCTL" --help 2>&1 | grep -q "completion"; then
    pass "Main help mentions completion"
else
    fail "Main help mentions completion"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "list"; then
    pass "Sonarr help mentions list"
else
    fail "Sonarr help mentions list"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "search"; then
    pass "Sonarr help mentions search"
else
    fail "Sonarr help mentions search"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "add"; then
    pass "Sonarr help mentions add"
else
    fail "Sonarr help mentions add"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "info"; then
    pass "Sonarr help mentions info"
else
    fail "Sonarr help mentions info"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "delete"; then
    pass "Sonarr help mentions delete"
else
    fail "Sonarr help mentions delete"
fi

if "$ARRCTL" sonarr --help 2>&1 | grep -q "calendar"; then
    pass "Sonarr help mentions calendar"
else
    fail "Sonarr help mentions calendar"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "list"; then
    pass "Radarr help mentions list"
else
    fail "Radarr help mentions list"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "search"; then
    pass "Radarr help mentions search"
else
    fail "Radarr help mentions search"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "add"; then
    pass "Radarr help mentions add"
else
    fail "Radarr help mentions add"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "info"; then
    pass "Radarr help mentions info"
else
    fail "Radarr help mentions info"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "delete"; then
    pass "Radarr help mentions delete"
else
    fail "Radarr help mentions delete"
fi

if "$ARRCTL" radarr --help 2>&1 | grep -q "calendar"; then
    pass "Radarr help mentions calendar"
else
    fail "Radarr help mentions calendar"
fi

if "$ARRCTL" calendar --help 2>&1 | grep -q "days"; then
    pass "Calendar help mentions days"
else
    fail "Calendar help mentions days"
fi

if "$ARRCTL" calendar --help 2>&1 | grep -q "week"; then
    pass "Calendar help mentions week"
else
    fail "Calendar help mentions week"
fi

if "$ARRCTL" tautulli --help 2>&1 | grep -q "now"; then
    pass "Tautulli help mentions now"
else
    fail "Tautulli help mentions now"
fi

if "$ARRCTL" overseerr --help 2>&1 | grep -q "pending"; then
    pass "Overseerr help mentions pending"
else
    fail "Overseerr help mentions pending"
fi

if "$ARRCTL" overseerr --help 2>&1 | grep -q "approve"; then
    pass "Overseerr help mentions approve"
else
    fail "Overseerr help mentions approve"
fi

if "$ARRCTL" overseerr --help 2>&1 | grep -q "deny"; then
    pass "Overseerr help mentions deny"
else
    fail "Overseerr help mentions deny"
fi

# Basic argument validation checks for new info/delete commands
printf '\n%s\n' "--- Info/Delete Argument Validation ---"

test_case_fail_contains "sonarr info missing selector fails" "Either --id or --name is required" \
    env SONARR_URL="http://127.0.0.1" SONARR_API_KEY="test" "$ARRCTL" sonarr info

test_case_fail_contains "sonarr info invalid --id fails" "--id must be a numeric Sonarr series ID" \
    env SONARR_URL="http://127.0.0.1" SONARR_API_KEY="test" "$ARRCTL" sonarr info --id abc

test_case_fail_contains "sonarr delete missing --id fails" "--id is required" \
    env SONARR_URL="http://127.0.0.1" SONARR_API_KEY="test" "$ARRCTL" sonarr delete --yes

test_case_fail_contains "sonarr delete invalid --id fails" "--id must be a numeric Sonarr series ID" \
    env SONARR_URL="http://127.0.0.1" SONARR_API_KEY="test" "$ARRCTL" sonarr delete --id abc --yes

test_case_fail_contains "radarr info missing selector fails" "Either --id or --name is required" \
    env RADARR_URL="http://127.0.0.1" RADARR_API_KEY="test" "$ARRCTL" radarr info

test_case_fail_contains "radarr info invalid --id fails" "--id must be a numeric Radarr movie ID" \
    env RADARR_URL="http://127.0.0.1" RADARR_API_KEY="test" "$ARRCTL" radarr info --id abc

test_case_fail_contains "radarr delete missing --id fails" "--id is required" \
    env RADARR_URL="http://127.0.0.1" RADARR_API_KEY="test" "$ARRCTL" radarr delete --yes

test_case_fail_contains "radarr delete invalid --id fails" "--id must be a numeric Radarr movie ID" \
    env RADARR_URL="http://127.0.0.1" RADARR_API_KEY="test" "$ARRCTL" radarr delete --id abc --yes

# Summary
printf '\n%s\n' "=== Summary ==="
printf 'Passed: %d\n' "$PASSED"
printf 'Failed: %d\n' "$FAILED"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

printf '\n%sAll tests passed!%s\n' "$GREEN" "$NC"
