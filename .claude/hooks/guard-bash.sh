#!/usr/bin/env bash
#
# guard-bash.sh — PreToolUse guardrail for Bash commands.
#
# Reads the proposed Bash command from stdin (Claude Code passes the tool
# input as JSON on stdin) and rejects two specific footguns:
#
#   1. Force-pushing to main / master (`git push --force[-with-lease] ... main|master`).
#   2. Recursive deletion of repo-rooted absolute paths (`rm -rf /Users/.../domjudge-tool*`).
#
# All other commands pass through. Exit 0 = allow, exit 2 = block.
#
# This is intentionally narrow — see .claude/settings.json for context.

set -uo pipefail

# Read the tool input JSON from stdin.
input=$(cat)

# Extract the command field. Tolerate jq absence by falling back to grep.
if command -v jq >/dev/null 2>&1; then
    cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty')
else
    # Best-effort: pull the first "command":"..." string. Imperfect, but enough
    # to fail-closed when jq is missing — we'd rather miss a guard than not
    # enforce one.
    cmd=$(printf '%s' "$input" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi

if [ -z "$cmd" ]; then
    # No command field — let Claude Code surface its own error.
    exit 0
fi

block() {
    # Stderr message is shown to the user; non-zero exit blocks the call.
    printf 'guard-bash.sh: blocked — %s\n' "$1" >&2
    printf 'command: %s\n' "$cmd" >&2
    exit 2
}

# 1) Force-push to main/master (covers --force and --force-with-lease).
if printf '%s' "$cmd" | grep -Eq 'git[[:space:]]+push([[:space:]]+[^|;&]*)?[[:space:]]+(--force([[:space:]]|=|$)|--force-with-lease([[:space:]]|=|$)|-f([[:space:]]|$))'; then
    if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]/:])(main|master)([[:space:]]|$)'; then
        block "force-pushing to main/master is not allowed; use a feature branch"
    fi
fi

# 2) rm -rf of any absolute path that points inside the project root or its parent.
if printf '%s' "$cmd" | grep -Eq 'rm[[:space:]]+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r|-rf|-fr|--recursive[[:space:]]+--force|--force[[:space:]]+--recursive)'; then
    if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]"'"'"'])/Users/[^[:space:]"'"'"']*domjudge-tool'; then
        block "rm -rf against an absolute repo path is not allowed; use git clean or remove specific files"
    fi
fi

exit 0
