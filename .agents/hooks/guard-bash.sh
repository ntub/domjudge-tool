#!/usr/bin/env bash
#
# guard-bash.sh - PreToolUse guardrail for shell commands.
# Adapted from .claude/hooks/guard-bash.sh for Antigravity CLI.
#
# Reads the proposed command from stdin (tool input as JSON) and rejects
# a few repository-level footguns:
#
#   1. Force-pushing to main/master.
#   2. Hard resets and broad git clean operations.
#   3. Recursive deletion of the repo root or broad repo globs.
#
# All other commands pass through. Exit 0 = allow, exit 2 = block.

set -uo pipefail

# Read the tool input JSON from stdin.
input=$(cat)

# Extract the command field. Support both Claude Code and Antigravity schemas.
if command -v jq >/dev/null 2>&1; then
    cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // .arguments.CommandLine // empty')
else
    # Best-effort: pull the first "command" or "CommandLine" string.
    cmd=$(printf '%s' "$input" | sed -n 's/.*"\(command\|CommandLine\)"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\2/p' | head -n1)
fi

if [ -z "$cmd" ]; then
    # No command field — let the agent surface its own error.
    exit 0
fi

block() {
    # Stderr is shown to the user; non-zero exit blocks the call.
    printf 'guard-bash.sh: blocked - %s\n' "$1" >&2
    printf 'command: %s\n' "$cmd" >&2
    exit 2
}

# 1) Force-push to main/master (covers --force and --force-with-lease).
if printf '%s' "$cmd" | grep -Eq 'git[[:space:]]+push([[:space:]]+[^|;&]*)?[[:space:]]+(--force([[:space:]]|=|$)|--force-with-lease([[:space:]]|=|$)|-f([[:space:]]|$))'; then
    if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]/:])(main|master)([[:space:]]|$)'; then
        block "force-pushing to main/master is not allowed; use a feature branch"
    fi
fi

# 2) Destructive git state operations.
if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]|;&])git[[:space:]]+reset[[:space:]]+--hard([[:space:]]|$)'; then
    block "git reset --hard is not allowed from the agent"
fi

if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]|;&])git[[:space:]]+clean[[:space:]]+(-[a-zA-Z]*f[a-zA-Z]*d|-d[[:space:]]+-f|-f[[:space:]]+-d)'; then
    block "git clean -fd style commands are not allowed from the agent"
fi

# 3) rm -rf of the repo root, broad repo globs, or repo-rooted absolute paths.
if printf '%s' "$cmd" | grep -Eq 'rm[[:space:]]+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r|-rf|-fr|--recursive[[:space:]]+--force|--force[[:space:]]+--recursive)'; then
    if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]"'"'"'])(\.|\./|\*|\./\*|/|[$][{]?CLAUDE_PROJECT_DIR[}]?)([[:space:]"'"'"']|$)'; then
        block "rm -rf against the repo root or broad globs is not allowed"
    fi
    if printf '%s' "$cmd" | grep -Eq '(^|[[:space:]"'"'"'])/Users/[^[:space:]"'"'"']*domjudge-tool'; then
        block "rm -rf against an absolute repo path is not allowed"
    fi
fi

exit 0
