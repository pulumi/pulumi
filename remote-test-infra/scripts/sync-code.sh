#!/usr/bin/env bash
# sync-code.sh - Rsync the working directory to ~/pulumi-work/ on the remote EC2 instance.
# Expects REMOTE_IP and REMOTE_KEY_FILE to be set (via get-config.sh).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load config if not already set.
if [[ -z "${REMOTE_IP:-}" || -z "${REMOTE_KEY_FILE:-}" ]]; then
    # shellcheck source=get-config.sh
    source "$SCRIPT_DIR/get-config.sh"
fi

WORK_DIR="${1:-$(pwd)}"
REMOTE_USER="testrunner"
REMOTE_PATH="~/pulumi-work/"

echo "Syncing ${WORK_DIR} to ${REMOTE_USER}@${REMOTE_IP}:${REMOTE_PATH} ..." >&2

rsync -az --delete \
    --exclude='.git/' \
    --exclude='node_modules/' \
    --exclude='bin/' \
    --exclude='.make/' \
    --exclude='__pycache__/' \
    -e "ssh -i ${REMOTE_KEY_FILE} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR" \
    "${WORK_DIR}/" \
    "${REMOTE_USER}@${REMOTE_IP}:${REMOTE_PATH}"

echo "Sync complete." >&2
