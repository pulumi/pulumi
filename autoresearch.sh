#!/usr/bin/env bash
# autoresearch.sh — Benchmark script for nodejs conformance tests
# Runs all 3 test variants (TSC, TSNode, Bun) on the remote EC2 instance.
# Outputs METRIC lines for the autoresearch loop.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_IP=$(cat ~/.cache/remote-test/nodejs-conformance-tests/instance-ip)
REMOTE_KEY_FILE="$HOME/.cache/remote-test/nodejs-conformance-tests/ssh-key.pem"

SSH_OPTS="-i $REMOTE_KEY_FILE -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

# Sync code to remote
echo "Syncing code to remote..." >&2
export REMOTE_IP REMOTE_KEY_FILE
"$SCRIPT_DIR/remote-test-infra/scripts/sync-code.sh" "$SCRIPT_DIR" >&2

# Build the test binary and test-language server once on remote
echo "Building test binaries on remote..." >&2
ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
    "cd ~/pulumi-work && mise exec -- make build 2>&1 | tail -5" >&2

# Run each test variant and capture timing
run_variant() {
    local variant="$1"
    local run_name="$2"

    echo "Running ${run_name}..." >&2
    local start_ms
    start_ms=$(date +%s%3N)

    # Run tests on remote, capture exit code
    local exit_code=0
    ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
        "cd ~/pulumi-work/sdk/nodejs/cmd/pulumi-language-nodejs && \
         PATH=~/pulumi-work/bin:\$PATH \
         mise exec -- go test . -v -count=1 -timeout 60m -run '${variant}' 2>&1" >&2 || exit_code=$?

    local end_ms
    end_ms=$(date +%s%3N)
    local duration_ms=$(( end_ms - start_ms ))

    echo "METRIC ${run_name}_ms=${duration_ms}"

    if [ $exit_code -ne 0 ]; then
        echo "${run_name} FAILED with exit code ${exit_code}" >&2
        return $exit_code
    fi
    echo "${run_name} completed in ${duration_ms}ms" >&2
}

# Overall timing
total_start_ms=$(date +%s%3N)

# Run all 3 variants sequentially (as they currently run)
run_variant "TestLanguageTSC" "tsc"
run_variant "TestLanguageTSNode" "tsnode"
run_variant "TestLanguageBun" "bun"

total_end_ms=$(date +%s%3N)
total_ms=$(( total_end_ms - total_start_ms ))

echo "METRIC total_ms=${total_ms}"
echo "All variants completed in ${total_ms}ms" >&2
