#!/usr/bin/env bash
# teardown.sh - Destroy remote test infrastructure and clean up.
# Usage: ./teardown.sh [stack-name]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default stack name to the current git worktree basename.
default_stack_name() {
    local worktree_path
    worktree_path="$(git rev-parse --show-toplevel 2>/dev/null)" || true
    if [[ -n "$worktree_path" ]]; then
        basename "$worktree_path"
    else
        echo "dev"
    fi
}

STACK_NAME="${1:-$(default_stack_name)}"
CACHE_DIR="${HOME}/.cache/remote-test/${STACK_NAME}"

echo "=== Remote Test Infrastructure Teardown (stack: ${STACK_NAME}) ==="

# Ensure Pulumi CLI is available.
if ! command -v pulumi &>/dev/null; then
    if [[ -x "${REPO_ROOT}/bin/pulumi" ]]; then
        export PATH="${REPO_ROOT}/bin:${PATH}"
    else
        echo "Error: Pulumi CLI not found. Build it with: cd $REPO_ROOT && mise exec -- make build" >&2
        exit 1
    fi
fi

# ── Destroy infrastructure ──────────────────────────────────────────────────
echo "Destroying Pulumi stack '${STACK_NAME}'..."
(cd "$SCRIPT_DIR" && pulumi stack select "$STACK_NAME" 2>/dev/null && pulumi destroy --yes) || {
    echo "Warning: pulumi destroy failed or stack not found." >&2
}

# Optionally remove the stack entirely.
read -r -p "Remove Pulumi stack '${STACK_NAME}' entirely? [y/N] " response
if [[ "$response" =~ ^[Yy]$ ]]; then
    (cd "$SCRIPT_DIR" && pulumi stack rm "$STACK_NAME" --yes) || true
fi

# ── Clean up cached credentials ─────────────────────────────────────────────
if [[ -d "$CACHE_DIR" ]]; then
    echo "Removing cached credentials at ${CACHE_DIR}..."
    rm -rf "$CACHE_DIR"
fi

echo ""
echo "=== Teardown Complete ==="
echo ""
echo "Remember to remove the PreToolUse hook from your .claude/settings.json if configured."
