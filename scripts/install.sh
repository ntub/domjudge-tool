#!/bin/sh
# domjudge-tool installer (macOS and Linux)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ntub/domjudge-tool/main/scripts/install.sh | sh
#
# Environment variables:
#   DOMJUDGE_TOOL_VERSION       Release tag to install (e.g. v1.8.0; default: latest)
#   DOMJUDGE_TOOL_INSTALL_DIR   Target directory for binary (default: $HOME/.local/bin)
#   DOMJUDGE_TOOL_NO_MODIFY_PATH Set to 1 to skip modifying shell rc files
#
# Integrity note:
#   Binaries are not OS-code-signed; integrity comes from the SHA-256 checksum in
#   checksums.txt and optional GitHub build provenance verification:
#     gh attestation verify <archive> --repo ntub/domjudge-tool
#   Downloads via curl/wget are not quarantined by macOS Gatekeeper.

set -eu

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Darwin)
    TARGET_OS="darwin"
    ;;
  Linux)
    TARGET_OS="linux"
    ;;
  *)
    printf 'Error: unsupported operating system %s. Only macOS and Linux are supported by install.sh.\n' "$OS" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    TARGET_ARCH="amd64"
    ;;
  aarch64|arm64)
    TARGET_ARCH="arm64"
    ;;
  *)
    printf 'Error: unsupported architecture %s. Only amd64/x86_64 and arm64/aarch64 are supported.\n' "$ARCH" >&2
    exit 1
    ;;
esac

ARCHIVE="domjudge-tool_${TARGET_OS}_${TARGET_ARCH}.tar.gz"
CHECKSUMS="checksums.txt"

if [ -n "${DOMJUDGE_TOOL_VERSION:-}" ]; then
  BASE_URL="https://github.com/ntub/domjudge-tool/releases/download/${DOMJUDGE_TOOL_VERSION}"
else
  BASE_URL="https://github.com/ntub/domjudge-tool/releases/latest/download"
fi

INSTALL_DIR="${DOMJUDGE_TOOL_INSTALL_DIR:-$HOME/.local/bin}"
INSTALL_DIR="${INSTALL_DIR%/}"

download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    printf 'Error: neither curl nor wget is installed.\n' >&2
    exit 1
  fi
}

compute_sha256() {
  target="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    raw="$(sha256sum "$target")"
    printf '%s' "${raw%% *}"
  elif command -v shasum >/dev/null 2>&1; then
    raw="$(shasum -a 256 "$target")"
    printf '%s' "${raw%% *}"
  else
    printf 'Error: neither sha256sum nor shasum is available for checksum verification.\n' >&2
    exit 1
  fi
}
TMPDIR_WORK="$(mktemp -d 2>/dev/null || mktemp -d -t 'domjudge-tool-install')"
cleanup() {
  rm -rf "$TMPDIR_WORK"
}
trap cleanup EXIT INT TERM HUP

printf 'Downloading %s...\n' "$ARCHIVE"
download "$BASE_URL/$ARCHIVE" "$TMPDIR_WORK/$ARCHIVE"

printf 'Downloading %s...\n' "$CHECKSUMS"
download "$BASE_URL/$CHECKSUMS" "$TMPDIR_WORK/$CHECKSUMS"

EXPECTED_SUM=""
while read -r sum fname || [ -n "${sum:-}" ]; do
  fname="${fname#\*}"
  if [ "$fname" = "$ARCHIVE" ]; then
    EXPECTED_SUM="$sum"
    break
  fi
done < "$TMPDIR_WORK/$CHECKSUMS"

case "$EXPECTED_SUM" in
  [0-9a-fA-F]*)
    if [ "${#EXPECTED_SUM}" -ne 64 ]; then
      printf 'Error: invalid SHA-256 length (%d) in %s for %s\n' "${#EXPECTED_SUM}" "$CHECKSUMS" "$ARCHIVE" >&2
      exit 1
    fi
    ;;
  *)
    printf 'Error: %s not found in %s\n' "$ARCHIVE" "$CHECKSUMS" >&2
    exit 1
    ;;
esac

EXPECTED_SUM="$(printf '%s' "$EXPECTED_SUM" | tr '[:upper:]' '[:lower:]')"
ACTUAL_SUM="$(compute_sha256 "$TMPDIR_WORK/$ARCHIVE" | tr '[:upper:]' '[:lower:]')"

if [ "$ACTUAL_SUM" != "$EXPECTED_SUM" ]; then
  printf 'Error: SHA-256 checksum mismatch for %s\n  expected: %s\n  actual:   %s\n' "$ARCHIVE" "$EXPECTED_SUM" "$ACTUAL_SUM" >&2
  exit 1
fi
printf 'Verified SHA-256 checksum (%s).\n' "$ACTUAL_SUM"

EXTRACT_DIR="$TMPDIR_WORK/extracted"
mkdir -p "$EXTRACT_DIR"
tar -xzf "$TMPDIR_WORK/$ARCHIVE" -C "$EXTRACT_DIR"

if [ ! -f "$EXTRACT_DIR/domjudge-tool" ]; then
  printf 'Error: domjudge-tool binary not found in %s\n' "$ARCHIVE" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
TMP_BIN="$INSTALL_DIR/.domjudge-tool.tmp.$$"
cp "$EXTRACT_DIR/domjudge-tool" "$TMP_BIN"
chmod 755 "$TMP_BIN"
mv -f "$TMP_BIN" "$INSTALL_DIR/domjudge-tool"

INSTALLED_VERSION="$("$INSTALL_DIR/domjudge-tool" version 2>/dev/null || echo "domjudge-tool")"
printf 'Installed %s to %s/domjudge-tool\n' "$INSTALLED_VERSION" "$INSTALL_DIR"

is_in_path() {
  dir="$1"
  case ":$PATH:" in
    *":$dir:"*) return 0 ;;
    *) return 1 ;;
  esac
}

REAL_INSTALL_DIR="$(cd "$INSTALL_DIR" 2>/dev/null && pwd || printf '%s' "$INSTALL_DIR")"
ON_PATH=0
if is_in_path "$INSTALL_DIR" || is_in_path "$REAL_INSTALL_DIR"; then
  ON_PATH=1
fi

if [ "$ON_PATH" -eq 1 ]; then
  printf '%s is already in your PATH.\n' "$INSTALL_DIR"
  exit 0
fi

if [ "${DOMJUDGE_TOOL_NO_MODIFY_PATH:-0}" = "1" ]; then
  printf 'DOMJUDGE_TOOL_NO_MODIFY_PATH=1 set; skipping PATH configuration.\n'
  printf "To use domjudge-tool in your current shell, run:\n  export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
  exit 0
fi

# The installer appends one block per install directory. A later block
# prepends its directory, so reinstalling elsewhere takes precedence.
append_rc() {
  rc_path="$1"
  rc_line="export PATH=\"$INSTALL_DIR:\$PATH\""
  mkdir -p "$(dirname "$rc_path")"
  if [ -f "$rc_path" ] && grep -Fx "$rc_line" "$rc_path" >/dev/null 2>&1; then
    printf 'PATH configuration already present in %s\n' "$rc_path"
    return 0
  fi
  printf '\n# domjudge-tool installer\n%s\n' "$rc_line" >> "$rc_path"
  printf 'Added %s to PATH in %s\n' "$INSTALL_DIR" "$rc_path"
}

append_fish_rc() {
  rc_path="$1"
  rc_line="fish_add_path --move \"$INSTALL_DIR\""
  mkdir -p "$(dirname "$rc_path")"
  if [ -f "$rc_path" ] && grep -Fx "$rc_line" "$rc_path" >/dev/null 2>&1; then
    printf 'PATH configuration already present in %s\n' "$rc_path"
    return 0
  fi
  printf '\n# domjudge-tool installer\n%s\n' "$rc_line" >> "$rc_path"
  printf 'Added %s to PATH in %s\n' "$INSTALL_DIR" "$rc_path"
}

shell_env="${SHELL:-}"
case "$shell_env" in
  *zsh*)
    append_rc "${ZDOTDIR:-$HOME}/.zshrc"
    ;;
  *bash*)
    append_rc "$HOME/.bashrc"
    if [ "$TARGET_OS" = "darwin" ]; then
      append_rc "$HOME/.bash_profile"
    fi
    ;;
  *fish*)
    append_fish_rc "$HOME/.config/fish/conf.d/domjudge-tool.fish"
    ;;
  *)
    append_rc "$HOME/.profile"
    ;;
esac

printf 'Restart your shell or open a new terminal for PATH changes to take effect.\n'
case "$shell_env" in
  *fish*)
    printf 'To use domjudge-tool in your current shell, run:\n  fish_add_path "%s"\n' "$INSTALL_DIR"
    ;;
  *)
    printf "To use domjudge-tool in your current shell, run:\n  export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
    ;;
esac
