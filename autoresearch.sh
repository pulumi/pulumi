#!/usr/bin/env bash
# autoresearch.sh — Benchmark script for nodejs conformance tests
# Runs all 3 test variants (TSC, TSNode, Bun) on the remote EC2 instance
# using tmpfs for temp dirs.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_IP=$(cat ~/.cache/remote-test/nodejs-conformance-tests/instance-ip)
REMOTE_KEY_FILE="$HOME/.cache/remote-test/nodejs-conformance-tests/ssh-key.pem"

SSH_OPTS="-i $REMOTE_KEY_FILE -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ServerAliveInterval=30 -o ServerAliveCountMax=10"

# Sync code to remote
echo "Syncing code to remote..." >&2
export REMOTE_IP REMOTE_KEY_FILE
"$SCRIPT_DIR/remote-test-infra/scripts/sync-code.sh" "$SCRIPT_DIR" >&2

# Ensure tmpfs is mounted for fast I/O
echo "Setting up tmpfs..." >&2
ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
    "mountpoint -q /mnt/test-tmpfs 2>/dev/null || (sudo mkdir -p /mnt/test-tmpfs && sudo mount -t tmpfs -o size=16G tmpfs /mnt/test-tmpfs && sudo chmod 777 /mnt/test-tmpfs)" >&2

# Build if needed
echo "Building..." >&2
ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
    "cd ~/pulumi-work && export PATH=/usr/local/go/bin:\$PATH && mise exec -- make build 2>&1 | tail -3" >&2

# Run all 3 variants simultaneously
echo "Running all 3 variants (TSC+TSNode+Bun) in parallel..." >&2
total_start_ms=$(date +%s%3N)

ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
    "cd ~/pulumi-work/sdk/nodejs/cmd/pulumi-language-nodejs && \
     export PATH=~/pulumi-work/bin:/usr/local/go/bin:\$PATH && \
     export TMPDIR=/mnt/test-tmpfs && \
     mise exec -- go test . -v -count=1 -timeout 60m -run 'TestLanguage(TSC|TSNode|Bun)' 2>&1" >&2

total_end_ms=$(date +%s%3N)
total_ms=$(( total_end_ms - total_start_ms ))

echo "METRIC total_ms=${total_ms}"
echo "All variants completed in ${total_ms}ms" >&2
