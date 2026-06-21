#!/usr/bin/env bash
#
# Format Go files after Antigravity edits them. Non-Go edits are ignored.
# Adapted from .claude/hooks/format-go.sh for Antigravity CLI.

set -u

input=$(cat)
project_dir=${CLAUDE_PROJECT_DIR:-$(pwd)}

cd "$project_dir" 2>/dev/null || exit 0

paths=""
if command -v jq >/dev/null 2>&1; then
    paths=$(printf '%s' "$input" | jq -r '
        [
            .tool_input.file_path?,
            (.tool_input.file_paths[]?),
            .tool_input.path?,
            (.tool_input.paths[]?),
            .arguments.TargetFile?,
            .arguments.AbsolutePath?
        ] | .[] | select(type == "string")
    ' 2>/dev/null || true)
fi

if [ -n "${CLAUDE_FILE_PATHS:-}" ]; then
    paths="${paths}
${CLAUDE_FILE_PATHS}"
fi

if [ -z "$paths" ]; then
    exit 0
fi

if command -v gofumpt >/dev/null 2>&1; then
    formatter=(gofumpt -w)
else
    formatter=(gofmt -s -w)
fi

printf '%s\n' "$paths" | while IFS= read -r path; do
    [ -n "$path" ] || continue

    case "$path" in
        *..* | .git/* | */.git/*)
            continue
            ;;
    esac

    case "$path" in
        *.go) ;;
        *) continue ;;
    esac

    case "$path" in
        "$project_dir"/*)
            rel=${path#"$project_dir"/}
            ;;
        /*)
            continue
            ;;
        *)
            rel=$path
            ;;
    esac

    [ -f "$rel" ] || continue
    "${formatter[@]}" "$rel" 2>/dev/null || true
done
