#!/bin/bash
# WorktreeRemove hook: tear down an agent worktree. Idempotent; exit 0 always.
cd "${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
WP=$(cat | jq -r '.worktree_path // empty')
[ -n "$WP" ] && git worktree remove --force "$WP" 2>/dev/null
exit 0
