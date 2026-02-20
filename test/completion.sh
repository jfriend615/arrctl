#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=/dev/null
source "${REPO_DIR}/completions/arrctl.bash"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

run_completion() {
  local -a words=("$@")
  COMP_WORDS=("${words[@]}")
  COMP_CWORD=$((${#words[@]} - 1))
  COMPREPLY=()
  _arrctl_completions
  printf '%s\n' "${COMPREPLY[@]}"
}

out="$(run_completion arrctl sonarr "")"
echo "$out" | grep -qx "list" || fail "arrctl sonarr <tab> should include 'list'"
echo "$out" | grep -qx "search" || fail "arrctl sonarr <tab> should include 'search'"

out="$(run_completion arrctl tautulli stale --)"
echo "$out" | grep -qx -- "--min-days" || fail "arrctl tautulli stale --<tab> should include --min-days"
echo "$out" | grep -qx -- "--library" || fail "arrctl tautulli stale --<tab> should include --library"

# Install command should be idempotent
TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

HOME="$TMP_HOME" SHELL="/bin/bash" "${REPO_DIR}/bin/arrctl" completion --install >/dev/null
HOME="$TMP_HOME" SHELL="/bin/bash" "${REPO_DIR}/bin/arrctl" completion --install >/dev/null

[ -f "$TMP_HOME/.bash_profile" ] || [ -f "$TMP_HOME/.bashrc" ] || fail "completion install should create bash profile file"
profile="$TMP_HOME/.bash_profile"
[ -f "$profile" ] || profile="$TMP_HOME/.bashrc"

count="$(grep -c "# >>> arrctl completion >>>" "$profile" || true)"
[ "$count" -eq 1 ] || fail "completion block should be added once (idempotent)"

echo "PASS: completion tests"
