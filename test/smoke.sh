#!/bin/sh
# smoke.sh - Basic sanity checks for arrctl
# POSIX compliant

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ARRCTL="${REPO_DIR}/bin/arrctl"

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

printf '%s\n\n' "=== arrctl Smoke Tests ==="

# Check dependencies
printf '%s\n' "--- Checking Dependencies ---"
test_case "curl is available" command -v curl
test_case "jq is available" command -v jq

# POSIX compliance checks
printf '\n%s\n' "--- POSIX Compliance ---"

if command -v shellcheck >/dev/null 2>&1; then
    test_case "shellcheck: bin/arrctl" shellcheck -s sh "${REPO_DIR}/bin/arrctl"
    test_case "shellcheck: lib/common.sh" shellcheck -s sh "${REPO_DIR}/lib/common.sh"
    test_case "shellcheck: lib/sonarr.sh" shellcheck -s sh "${REPO_DIR}/lib/sonarr.sh"
else
    skip "shellcheck not installed"
fi

if command -v dash >/dev/null 2>&1; then
    test_case "dash -n: bin/arrctl" dash -n "${REPO_DIR}/bin/arrctl"
    test_case "dash -n: lib/common.sh" dash -n "${REPO_DIR}/lib/common.sh"
    test_case "dash -n: lib/sonarr.sh" dash -n "${REPO_DIR}/lib/sonarr.sh"
else
    skip "dash not installed"
fi

# Help output checks
printf '\n%s\n' "--- Help Commands ---"
test_case "arrctl --help works" "$ARRCTL" --help
test_case "arrctl --version works" "$ARRCTL" --version
test_case "arrctl sonarr --help works" "$ARRCTL" sonarr --help
test_case "arrctl sonarr help works" "$ARRCTL" sonarr help

# Invalid command handling
printf '\n%s\n' "--- Error Handling ---"
test_case_fail "arrctl invalid-command fails" "$ARRCTL" invalid-command
test_case_fail "arrctl sonarr invalid-subcommand fails" "$ARRCTL" sonarr invalid-subcommand

# Help output contains expected content
printf '\n%s\n' "--- Help Content ---"
if "$ARRCTL" --help 2>&1 | grep -q "sonarr"; then
    pass "Main help mentions sonarr"
else
    fail "Main help mentions sonarr"
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

# Summary
printf '\n%s\n' "=== Summary ==="
printf 'Passed: %d\n' "$PASSED"
printf 'Failed: %d\n' "$FAILED"

if [ "$FAILED" -gt 0 ]; then
    exit 1
fi

printf '\n%sAll tests passed!%s\n' "$GREEN" "$NC"
