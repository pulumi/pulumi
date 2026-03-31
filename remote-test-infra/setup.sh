#!/usr/bin/env bash
# setup.sh - One-command setup for remote test execution on EC2.
# Usage: ./setup.sh <stack-name> [--region <aws-region>]
# Builds Pulumi CLI from the repo, provisions EC2 instance, configures hooks.
set -euo pipefail

usage() {
    echo "Usage: $0 [stack-name] [--region <aws-region>]"
    echo ""
    echo "  stack-name    Name for the Pulumi stack (default: current git worktree name)"
    echo "  --region      AWS region (default: us-east-1)"
    exit 1
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    usage
fi

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

# First non-flag argument is the stack name; default to worktree name.
STACK_NAME=""
AWS_REGION="us-east-1"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --region) AWS_REGION="$2"; shift 2 ;;
        --*) echo "Unknown option: $1" >&2; usage ;;
        *) STACK_NAME="$1"; shift ;;
    esac
done

if [[ -z "$STACK_NAME" ]]; then
    STACK_NAME="$(default_stack_name)"
    echo "No stack name provided, using worktree name: ${STACK_NAME}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CACHE_DIR="${HOME}/.cache/remote-test/${STACK_NAME}"

echo "=== Remote Test Infrastructure Setup (stack: ${STACK_NAME}) ==="

# ── Step 1: Ensure Pulumi CLI is available ──────────────────────────────────
if ! command -v pulumi &>/dev/null; then
    echo "Pulumi CLI not found. Building from repo..."
    (cd "$REPO_ROOT" && mise exec -- make build)
    export PATH="${REPO_ROOT}/bin:${PATH}"
    if ! command -v pulumi &>/dev/null; then
        echo "Error: Failed to build pulumi CLI" >&2
        exit 1
    fi
    echo "Pulumi CLI built: $(pulumi version)"
else
    echo "Pulumi CLI found: $(pulumi version)"
fi

# ── Step 2: Ensure Node.js is available ─────────────────────────────────────
if ! command -v node &>/dev/null; then
    echo "Error: Node.js is required. Install it or activate mise." >&2
    exit 1
fi

# ── Step 3: Install npm dependencies ────────────────────────────────────────
echo "Installing npm dependencies..."
(cd "$SCRIPT_DIR" && npm install)

# ── Step 4: Initialize Pulumi stack if needed ───────────────────────────────
(cd "$SCRIPT_DIR" && pulumi stack select "$STACK_NAME" 2>/dev/null || pulumi stack init "$STACK_NAME")

# Set AWS region
echo "Setting AWS region to ${AWS_REGION}"
(cd "$SCRIPT_DIR" && pulumi config set aws:region "$AWS_REGION")

# ── Step 5: Deploy infrastructure ───────────────────────────────────────────
echo "Deploying EC2 instance..."
(cd "$SCRIPT_DIR" && pulumi up --yes)

# ── Step 6: Cache SSH key and instance IP ───────────────────────────────────
mkdir -p "$CACHE_DIR"

INSTANCE_IP="$(cd "$SCRIPT_DIR" && pulumi stack output instanceIp)"
PRIVATE_KEY="$(cd "$SCRIPT_DIR" && pulumi stack output privateKey --show-secrets)"

echo "$INSTANCE_IP" > "$CACHE_DIR/instance-ip"
echo "$PRIVATE_KEY" > "$CACHE_DIR/ssh-key.pem"
chmod 600 "$CACHE_DIR/ssh-key.pem"

echo "Instance IP: $INSTANCE_IP"
echo "SSH key cached at: $CACHE_DIR/ssh-key.pem"

# ── Step 7: Wait for instance to be ready ───────────────────────────────────
echo "Waiting for instance to finish bootstrapping..."
MAX_ATTEMPTS=60
ATTEMPT=0
while (( ATTEMPT < MAX_ATTEMPTS )); do
    ATTEMPT=$((ATTEMPT + 1))
    if ssh -i "$CACHE_DIR/ssh-key.pem" \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o LogLevel=ERROR \
        -o ConnectTimeout=5 \
        "testrunner@${INSTANCE_IP}" \
        "test -f /tmp/userdata-complete" 2>/dev/null; then
        echo "Instance is ready! (attempt ${ATTEMPT}/${MAX_ATTEMPTS})"
        break
    fi
    if (( ATTEMPT >= MAX_ATTEMPTS )); then
        echo "Error: Instance did not become ready within $((MAX_ATTEMPTS * 10)) seconds." >&2
        echo "Check /var/log/userdata.log on the instance for details." >&2
        exit 1
    fi
    echo "  Waiting... (attempt ${ATTEMPT}/${MAX_ATTEMPTS})"
    sleep 10
done

# ── Step 8: Initial code sync ───────────────────────────────────────────────
echo "Performing initial code sync..."
export REMOTE_IP="$INSTANCE_IP"
export REMOTE_KEY_FILE="$CACHE_DIR/ssh-key.pem"
"$SCRIPT_DIR/scripts/sync-code.sh" "$REPO_ROOT"

# ── Step 9: Build Pulumi on remote instance ─────────────────────────────────
echo "Building Pulumi CLI on remote instance..."
ssh -i "$CACHE_DIR/ssh-key.pem" \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o LogLevel=ERROR \
    "testrunner@${INSTANCE_IP}" \
    "cd ~/pulumi-work && mise install && mise exec -- make build && echo 'export PATH=\$HOME/pulumi-work/bin:\$PATH' >> ~/.bashrc"

# ── Done ────────────────────────────────────────────────────────────────────
echo ""
echo "=== Setup Complete ==="
echo ""
echo "Instance IP:  $INSTANCE_IP"
echo "SSH command:  ssh -i $CACHE_DIR/ssh-key.pem testrunner@$INSTANCE_IP"
echo ""
echo "To enable the Claude Code hook, add this to your .claude/settings.json:"
echo ""
echo "  {\"hooks\": {\"PreToolUse\": [{\"matcher\": \"Bash\", \"hooks\": [{\"type\": \"command\", \"command\": \"${SCRIPT_DIR}/scripts/remote-test.sh\"}]}]}}"
echo ""
echo "To tear down: ${SCRIPT_DIR}/teardown.sh"
