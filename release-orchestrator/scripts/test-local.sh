#!/usr/bin/env bash
#
# Spin up Step Functions Local and run the integration tests.
#
# Prereq: Docker daemon running.
#
# Usage:
#   ./scripts/test-local.sh         # run integration tests
#   ./scripts/test-local.sh shell   # leave the container running for ad-hoc

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MOCK_FILE="$REPO_ROOT/tests/integration/MockConfigFile.json"
CONTAINER_NAME="release-orch-sfn-local"
PORT="${SFN_LOCAL_PORT:-8083}"

cleanup() {
    if docker ps -q --filter "name=$CONTAINER_NAME" | grep -q .; then
        docker stop "$CONTAINER_NAME" >/dev/null
    fi
}
trap cleanup EXIT

# If a container is already running (left over from a previous run), reuse it.
if ! docker ps -q --filter "name=$CONTAINER_NAME" | grep -q .; then
    echo "starting amazon/aws-stepfunctions-local on :$PORT"
    docker run --rm -d \
        --name "$CONTAINER_NAME" \
        -p "$PORT":8083 \
        -v "$MOCK_FILE":/home/StepFunctionsLocal/MockConfigFile.json \
        -e SFN_MOCK_CONFIG=/home/StepFunctionsLocal/MockConfigFile.json \
        amazon/aws-stepfunctions-local

    # Wait for SFN Local to come up
    for i in {1..30}; do
        if curl -fs "http://localhost:$PORT" >/dev/null 2>&1 \
           || curl -fs "http://localhost:$PORT/" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
fi

if [ "${1:-}" = "shell" ]; then
    echo "container running on http://localhost:$PORT"
    echo "press ctrl-c to stop"
    trap - EXIT
    docker logs -f "$CONTAINER_NAME"
    exit 0
fi

export SFN_LOCAL_ENDPOINT="http://localhost:$PORT"
cd "$REPO_ROOT"
uv run pytest tests/integration -v
