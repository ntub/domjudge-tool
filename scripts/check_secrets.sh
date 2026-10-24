#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
GIT_COMMON_DIR="$(git -C "$ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
RESULTS="${TRUFFLEHOG_RESULTS:-verified,unknown}"

FILESYSTEM_PATHS=(
    .github
    cmd
    docs
    internal
    scripts
    testdata
    AGENTS.md
    CHANGELOG.md
    CLAUDE.md
    CONTRIBUTING.md
    LICENSE
    README.md
    cliff.toml
    go.mod
    go.sum
    lefthook.yml
    mise.lock
    mise.toml
)

if ! command -v trufflehog >/dev/null 2>&1; then
    cat >&2 <<'EOF'
check_secrets.sh: trufflehog is required.

Install the pinned version with `mise install`, then run `mise run secret-scan`.
EOF
    exit 2
fi
TRUFFLEHOG=(trufflehog --no-update)
if [[ -n "$GIT_COMMON_DIR" ]]; then
    GIT_TARGET="file://$(dirname "$GIT_COMMON_DIR")"
else
    GIT_TARGET="file://."
fi

cd "$ROOT"

printf 'trufflehog filesystem scan (%s)\n' "$RESULTS"
"${TRUFFLEHOG[@]}" filesystem "${FILESYSTEM_PATHS[@]}" --results="$RESULTS" --fail

printf 'trufflehog git history scan (%s)\n' "$RESULTS"
"${TRUFFLEHOG[@]}" git "$GIT_TARGET" --results="$RESULTS" --fail
