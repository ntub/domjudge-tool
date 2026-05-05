#!/usr/bin/env bash
set -euo pipefail

RAW_DIR="${RAW_DIR:-tmp/raw_api_v4}"
DJ_CONTEST_ID="${DJ_CONTEST_ID:-2}"
DJ_TEAM_ID="${DJ_TEAM_ID:-2}"
DJ_ADMIN_USER_ID="${DJ_ADMIN_USER_ID:-1}"
DJ_USER_WITH_EMAIL_ID="${DJ_USER_WITH_EMAIL_ID:-}"
DJ_SUBMISSION_ID="${DJ_SUBMISSION_ID:-1}"

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        printf 'capture_api_fixtures.sh: %s is required\n' "$name" >&2
        exit 2
    fi
}

require_command() {
    local name="$1"
    if ! command -v "$name" >/dev/null 2>&1; then
        printf 'capture_api_fixtures.sh: %s is required\n' "$name" >&2
        exit 2
    fi
}

json_pretty_print() {
    python3 -m json.tool "$1" >"$2"
}

curl_json() {
    local url="$1"
    local out="$2"
    local tmp
    tmp="$(mktemp)"
    if ! curl --fail --silent --show-error --netrc-file "$NETRC_FILE" "$url" -o "$tmp"; then
        rm -f "$tmp"
        return 1
    fi
    mkdir -p "$(dirname "$out")"
    if ! json_pretty_print "$tmp" "$out"; then
        rm -f "$tmp"
        return 1
    fi
    rm -f "$tmp"
    printf 'captured %s\n' "$out"
}

curl_json_optional() {
    local url="$1"
    local out="$2"
    if ! curl_json "$url" "$out"; then
        printf 'warning: optional capture failed: %s\n' "$url" >&2
        rm -f "$out"
    fi
}

curl_bin_optional() {
    local url="$1"
    local out="$2"
    mkdir -p "$(dirname "$out")"
    if curl --fail --silent --show-error --netrc-file "$NETRC_FILE" "$url" -o "$out"; then
        printf 'captured %s\n' "$out"
    else
        printf 'warning: optional capture failed: %s\n' "$url" >&2
        rm -f "$out"
    fi
}

require_env DJ_BASE
require_env DJ_USER
require_env DJ_PASS
require_command curl
require_command python3

DJ_BASE="${DJ_BASE%/}"
NETRC_FILE="$(mktemp)"
trap 'rm -f "$NETRC_FILE"' EXIT

host="$(python3 -c 'from urllib.parse import urlsplit; import sys; print(urlsplit(sys.argv[1]).hostname or "")' "$DJ_BASE")"
if [[ -z "$host" ]]; then
    printf 'capture_api_fixtures.sh: DJ_BASE must include a host\n' >&2
    exit 2
fi

cat >"$NETRC_FILE" <<EOF
machine $host
  login $DJ_USER
  password $DJ_PASS
EOF
chmod 600 "$NETRC_FILE"

mkdir -p "$RAW_DIR"

curl_json "$DJ_BASE/api/v4/version" "$RAW_DIR/version.json"
curl_json "$DJ_BASE/api/v4/info" "$RAW_DIR/info.json"
curl_json "$DJ_BASE/api/v4/user" "$RAW_DIR/user.json"

curl_json "$DJ_BASE/api/v4/users" "$RAW_DIR/users/list_default.json"
curl_json "$DJ_BASE/api/v4/users?team_id=$DJ_TEAM_ID" "$RAW_DIR/users/list_with_team_id.json"
curl_json "$DJ_BASE/api/v4/users/$DJ_ADMIN_USER_ID" "$RAW_DIR/users/single_admin.json"
if [[ -n "$DJ_USER_WITH_EMAIL_ID" ]]; then
    curl_json "$DJ_BASE/api/v4/users/$DJ_USER_WITH_EMAIL_ID" "$RAW_DIR/users/single_with_email.json"
fi

curl_json "$DJ_BASE/api/v4/contests" "$RAW_DIR/contests/list.json"

contest_dir="$RAW_DIR/contest_$DJ_CONTEST_ID"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/teams" "$contest_dir/teams.json"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/organizations" "$contest_dir/organizations.json"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/groups" "$contest_dir/groups.json"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/problems" "$contest_dir/problems.json"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/scoreboard" "$contest_dir/scoreboard_default.json"
curl_json "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/submissions" "$contest_dir/submissions.json"
curl_bin_optional \
    "$DJ_BASE/api/v4/contests/$DJ_CONTEST_ID/submissions/$DJ_SUBMISSION_ID/files" \
    "$contest_dir/submission_${DJ_SUBMISSION_ID}_files.zip"

if [[ -n "${DJ_FREEZE_CONTEST_ID:-}" ]]; then
    curl_json \
        "$DJ_BASE/api/v4/contests/$DJ_FREEZE_CONTEST_ID/scoreboard" \
        "$RAW_DIR/contest_$DJ_FREEZE_CONTEST_ID/scoreboard_with_freeze.json"
fi

if [[ -n "${DJ_EMPTY_SUBMISSIONS_CONTEST_ID:-}" ]]; then
    curl_json \
        "$DJ_BASE/api/v4/contests/$DJ_EMPTY_SUBMISSIONS_CONTEST_ID/submissions" \
        "$RAW_DIR/contest_$DJ_EMPTY_SUBMISSIONS_CONTEST_ID/submissions_empty.json"
fi

printf '\nraw API fixtures written to %s\n' "$RAW_DIR"
printf 'next: python3 scripts/sanitize_api.py %s testdata/api/v4\n' "$RAW_DIR"
