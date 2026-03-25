#!/usr/bin/env bash
# get-config.sh - Retrieve and cache EC2 instance config from Pulumi stack outputs.
# Writes instance IP and SSH key to ~/.cache/remote-test/<stack>/
# Usage: source this script or call it directly; it exports REMOTE_IP and REMOTE_KEY_FILE.
# The stack name defaults to the current git worktree basename.

set -euo pipefail

# Determine stack name from worktree.
_get_stack_name() {
    local worktree_path
    worktree_path="$(git rev-parse --show-toplevel 2>/dev/null)" || true
    if [[ -n "$worktree_path" ]]; then
        basename "$worktree_path"
    else
        echo "dev"
    fi
}

REMOTE_TEST_STACK="${REMOTE_TEST_STACK:-$(_get_stack_name)}"
CACHE_DIR="${HOME}/.cache/remote-test/${REMOTE_TEST_STACK}"
IP_FILE="${CACHE_DIR}/instance-ip"
KEY_FILE="${CACHE_DIR}/ssh-key.pem"

# If cache exists and is less than 1 hour old, reuse it.
if [[ -f "$IP_FILE" && -f "$KEY_FILE" ]]; then
    ip_age=$(( $(date +%s) - $(stat -c %Y "$IP_FILE") ))
    if (( ip_age < 3600 )); then
        export REMOTE_IP
        REMOTE_IP="$(cat "$IP_FILE")"
        export REMOTE_KEY_FILE="$KEY_FILE"
        return 0 2>/dev/null || exit 0
    fi
fi

mkdir -p "$CACHE_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Retrieve stack outputs from the Pulumi stack.
if ! command -v pulumi &>/dev/null; then
    echo "Error: pulumi CLI not found on PATH" >&2
    exit 1
fi

INSTANCE_IP="$(cd "$INFRA_DIR" && pulumi stack output instanceIp --stack "$REMOTE_TEST_STACK" 2>/dev/null)" || {
    echo "Error: failed to get instanceIp from Pulumi stack '${REMOTE_TEST_STACK}'" >&2
    exit 1
}

PRIVATE_KEY="$(cd "$INFRA_DIR" && pulumi stack output privateKey --show-secrets --stack "$REMOTE_TEST_STACK" 2>/dev/null)" || {
    echo "Error: failed to get privateKey from Pulumi stack '${REMOTE_TEST_STACK}'" >&2
    exit 1
}

echo "$INSTANCE_IP" > "$IP_FILE"
echo "$PRIVATE_KEY" > "$KEY_FILE"
chmod 600 "$KEY_FILE"

export REMOTE_IP="$INSTANCE_IP"
export REMOTE_KEY_FILE="$KEY_FILE"
