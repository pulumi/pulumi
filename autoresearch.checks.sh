#!/usr/bin/env bash
# autoresearch.checks.sh — Correctness check for conformance test changes.
# The benchmark script already runs the full test suite and fails on test failures,
# so this script just verifies the code compiles and basic sanity.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_IP=$(cat ~/.cache/remote-test/nodejs-conformance-tests/instance-ip)
REMOTE_KEY_FILE="$HOME/.cache/remote-test/nodejs-conformance-tests/ssh-key.pem"

SSH_OPTS="-i $REMOTE_KEY_FILE -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

# Sync code
export REMOTE_IP REMOTE_KEY_FILE
"$SCRIPT_DIR/remote-test-infra/scripts/sync-code.sh" "$SCRIPT_DIR" >&2

# Verify it compiles
echo "Verifying build compiles..." >&2
ssh $SSH_OPTS "testrunner@${REMOTE_IP}" \
    "cd ~/pulumi-work && mise exec -- go build ./pkg/testing/pulumi-test-language/... && mise exec -- go build ./sdk/nodejs/cmd/pulumi-language-nodejs/..." >&2

echo "Build verification passed." >&2
