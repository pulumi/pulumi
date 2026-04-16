#!/usr/bin/env bash
# remote-test.sh - Claude Code PreToolUse hook for the Bash tool.
# Intercepts test commands and runs them on a remote EC2 instance.
#
# Hook protocol:
#   - Reads JSON from stdin: {"tool_input": {"command": "..."}}
#   - Exit 0: allow local execution (non-test commands)
#   - Exit 2: block local execution (test commands run remotely instead)
#   - Messages on stderr are shown to Claude as feedback.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Read the hook input from stdin.
INPUT="$(cat)"

# Extract the command from the JSON payload.
COMMAND="$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)" || true

if [[ -z "$COMMAND" ]]; then
    # No command found in input; allow local execution.
    exit 0
fi

# Detect test commands.
is_test_command() {
    local cmd="$1"
    # Match common test invocations.
    if echo "$cmd" | grep -qE '(^|\s|&&|\|\||;)(go\s+test|make\s+(test_fast|test_all|test)\b|pytest|npm\s+test)'; then
        return 0
    fi
    return 1
}

if ! is_test_command "$COMMAND"; then
    # Not a test command; allow local execution.
    exit 0
fi

echo ">>> Intercepted test command, running remotely..." >&2
echo ">>> Command: ${COMMAND}" >&2

# Load remote instance config.
# shellcheck source=get-config.sh
source "$SCRIPT_DIR/get-config.sh"

# Sync the working directory to the remote instance.
"$SCRIPT_DIR/sync-code.sh"

REMOTE_USER="testrunner"

echo ">>> Running test on remote instance ${REMOTE_IP}..." >&2

# Run the command remotely in the synced directory.
# Use -t for pseudo-terminal so we get live output, and pipe stderr back.
ssh -i "$REMOTE_KEY_FILE" \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -o LogLevel=ERROR \
    "${REMOTE_USER}@${REMOTE_IP}" \
    "cd ~/pulumi-work && ${COMMAND}" >&2

EXIT_CODE=$?

if [[ $EXIT_CODE -ne 0 ]]; then
    echo ">>> Remote test exited with code ${EXIT_CODE}" >&2
else
    echo ">>> Remote test passed." >&2
fi

# Exit 2 to block local execution (the test already ran remotely).
exit 2
