#!/usr/bin/env bash
# Claude Code PostToolUse hook (docs/method.md, ported from kube-ward).
# After an edit to Go files, run `make verify` so validation
# can't be silently skipped in-session. No-ops for non-code edits (e.g. docs).
#
# Receives the tool-use payload as JSON on stdin. On failure, `make verify`'s
# output goes to stderr, which Claude Code surfaces back as feedback.
set -euo pipefail

payload="$(cat)"

# Extract the edited file path from the payload (portable; no jq dependency).
file="$(printf '%s' "$payload" \
  | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  | head -n1)"

case "$file" in
  *.go|go.mod|*/go.mod|go.sum|*/go.sum)
    # Verify the tree the file actually lives in, not CLAUDE_PROJECT_DIR: a git
    # WORKTREE checkout is its own tree, and running make in the primary checkout
    # validates code the edit never touched (found 2026-09-27 — the hook kept
    # re-linting the unchanged main checkout while a story branch evolved in a
    # worktree, failing on the main checkout's environment and never compiling
    # the edited files at all).
    repo="$(cd "$(dirname "$file")" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null || true)"
    cd "${repo:-${CLAUDE_PROJECT_DIR:-.}}"
    echo "▶ verify-hook: '$file' changed — running 'make verify' in $PWD" >&2
    make verify >&2
    ;;
  *)
    : # non-code edit — nothing to validate
    ;;
esac
