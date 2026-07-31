#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

FAKEBIN="$TMP_ROOT/fakebin"
PAYLOAD="$TMP_ROOT/payload"
HOME_DIR="$TMP_ROOT/home"
BIN_DIR="$TMP_ROOT/bin"
mkdir -p "$FAKEBIN" "$PAYLOAD" "$HOME_DIR" "$BIN_DIR"

VERSION_NO_V="1.2.3"
VERSION_TAG="v${VERSION_NO_V}"
ARCHIVE_BASENAME="arrctl_${VERSION_NO_V}_darwin_arm64"
ARCHIVE_NAME="${ARCHIVE_BASENAME}.tar.gz"
ARCHIVE_DIR="$PAYLOAD/$ARCHIVE_BASENAME"
ARCHIVE_PATH="$PAYLOAD/$ARCHIVE_NAME"
mkdir -p "$ARCHIVE_DIR"
printf '#!/bin/sh\necho arrctl %s\n' "$VERSION_TAG" > "$ARCHIVE_DIR/arrctl"
chmod +x "$ARCHIVE_DIR/arrctl"
tar -czf "$ARCHIVE_PATH" -C "$PAYLOAD" "$ARCHIVE_BASENAME"
if command -v shasum >/dev/null 2>&1; then
  SUM="$(shasum -a 256 "$ARCHIVE_PATH" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  SUM="$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')"
else
  echo "FAIL: smoke test requires shasum or sha256sum" >&2
  exit 1
fi
printf '%s  %s\n' "$SUM" "$ARCHIVE_NAME" > "$PAYLOAD/SHA256SUMS"

cat > "$FAKEBIN/uname" <<'EOF'
#!/bin/sh
if [ "$1" = "-s" ]; then
  echo Darwin
elif [ "$1" = "-m" ]; then
  echo arm64
else
  /usr/bin/uname "$@"
fi
EOF

cat > "$FAKEBIN/tput" <<'EOF'
#!/bin/sh
exit 1
EOF

cat > "$FAKEBIN/curl" <<EOF
#!/bin/sh
set -eu
out=""
url=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in
    -o)
      out="\$2"
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="\$1"
      shift
      ;;
  esac
done
case "\$url" in
  */$ARCHIVE_NAME) cp "$ARCHIVE_PATH" "\$out" ;;
  */SHA256SUMS) cp "$PAYLOAD/SHA256SUMS" "\$out" ;;
  *) exit 1 ;;
esac
EOF

chmod +x "$FAKEBIN/uname" "$FAKEBIN/tput" "$FAKEBIN/curl"

PATH="$FAKEBIN:/usr/bin:/bin:/usr/sbin:/sbin" \
HOME="$HOME_DIR" \
XDG_CONFIG_HOME="$HOME_DIR/.config" \
BIN_DIR="$BIN_DIR" \
VERSION="$VERSION_TAG" \
sh "$REPO_DIR/install.sh" >/dev/null

[ -x "$BIN_DIR/arrctl" ] || {
  echo "FAIL: install did not create executable" >&2
  exit 1
}

[ -f "$HOME_DIR/.config/arrctl/config.json" ] || {
  echo "FAIL: install did not create config template" >&2
  exit 1
}

[ ! -e "$BIN_DIR/.arrctl-install.$$" ] || {
  echo "FAIL: installer left staging file behind" >&2
  exit 1
}

NO_HASH_BIN="$TMP_ROOT/no-hash-bin"
NO_HASH_LOG="$TMP_ROOT/no-hash.log"
CURL_MARKER="$TMP_ROOT/curl-called"
mkdir -p "$NO_HASH_BIN"

cat > "$NO_HASH_BIN/curl" <<EOF
#!/bin/sh
touch "$CURL_MARKER"
exit 1
EOF

cat > "$NO_HASH_BIN/tar" <<'EOF'
#!/bin/sh
exit 0
EOF

chmod +x "$NO_HASH_BIN/curl" "$NO_HASH_BIN/tar"

if PATH="$NO_HASH_BIN" /bin/sh "$REPO_DIR/install.sh" >"$NO_HASH_LOG" 2>&1; then
  echo "FAIL: installer succeeded without a checksum tool" >&2
  exit 1
fi

grep -q 'shasum-or-sha256sum' "$NO_HASH_LOG" || {
  echo "FAIL: installer did not report missing checksum tool" >&2
  exit 1
}

[ ! -e "$CURL_MARKER" ] || {
  echo "FAIL: installer attempted a download before checksum preflight" >&2
  exit 1
}

echo "PASS: install smoke test"
