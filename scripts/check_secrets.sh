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
    Justfile
    LICENSE
    README.md
    cliff.toml
    go.mod
    go.sum
    lefthook.yml
)

if command -v trufflehog >/dev/null 2>&1; then
    TRUFFLEHOG=(trufflehog --no-update)
    if [[ -n "$GIT_COMMON_DIR" ]]; then
        GIT_TARGET="file://$(dirname "$GIT_COMMON_DIR")"
    else
        GIT_TARGET="file://."
    fi
elif command -v docker >/dev/null 2>&1; then
    TRUFFLEHOG=(
        docker run --rm
        -v "$ROOT:/repo:ro"
        -w /repo
        trufflesecurity/trufflehog:latest
    )
    GIT_TARGET="file:///repo"
else
    cat >&2 <<'EOF'
check_secrets.sh: trufflehog is required.

Install TruffleHog or run Docker so the official image can be used:
  curl -sSfL https://raw.githubusercontent.com/trufflesecurity/trufflehog/main/scripts/install.sh | sh -s -- -b /usr/local/bin
EOF
    exit 2
fi

cd "$ROOT"

printf 'trufflehog filesystem scan (%s)\n' "$RESULTS"
"${TRUFFLEHOG[@]}" filesystem "${FILESYSTEM_PATHS[@]}" --results="$RESULTS" --fail

printf 'trufflehog git history scan (%s)\n' "$RESULTS"
"${TRUFFLEHOG[@]}" git "$GIT_TARGET" --results="$RESULTS" --fail
