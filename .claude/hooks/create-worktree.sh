#!/bin/bash
# WorktreeCreate hook: make a git worktree for agent isolation.
# Reads JSON on stdin ({worktree_path, worktree_name, base_branch, ...});
# prints the created worktree path on stdout; exit 0 on success.
set -e
cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
json="$(cat)"
WP=$(printf '%s' "$json" | jq -r '.worktree_path // empty')
WN=$(printf '%s' "$json" | jq -r '.worktree_name // "agent-wt"')
BB=$(printf '%s' "$json" | jq -r '.base_branch // "HEAD"')
[ -n "$WP" ] || { echo "create-worktree: missing worktree_path" >&2; exit 1; }
git rev-parse --verify "$BB" >/dev/null 2>&1 || BB=HEAD
git worktree add -B "$WN" "$WP" "$BB" >&2
echo "$WP"
