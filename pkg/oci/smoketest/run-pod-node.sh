#!/usr/bin/env bash
#
# Language-image smoke test (design Phase 6 on-ramp): a *Node* program runs as a
# container in the pod. Unlike the Go demos (a static binary), this is a real
# language image — Node base + @pulumi/pulumi via npm + an entrypoint shim that
# bootstraps the runtime from PULUMI_* env vars (program-node/oci-bootstrap.sh),
# replicating what pulumi-language-nodejs would pass. A successful run proves the
# language-image + env-bootstrap contract for a non-Go language: the Node runtime
# connects to the engine's monitor, runs, and reports a stack output.
#
# Pipeline: cross-compile this branch's CLI + OCI host, build the engine image,
# build the Node program image, run `pulumi up` in the engine container, and
# assert the program's stack output.
#
# Usage: run-pod-node.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-node"
PROGRAM_DIR="$SMOKE_DIR/program-node"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
EXPECTED_GREETING="hello-from-node-in-a-pod"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run language-image test"
  exit 1
fi

echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

echo "==> building engine image $ENGINE_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"

echo "==> building Node program image $PROGRAM_IMAGE (npm install @pulumi/pulumi)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME (Node program runs as a sibling container)"
docker run --rm -i \
  --privileged \
  --network "$NET" \
  --name "$ENGINE_NAME" \
  --hostname "$ENGINE_NAME" \
  --label "$POD_LABEL" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$WORK/project":/project \
  -v "$WORK/state":/state \
  -w /project \
  -e PULUMI_POD_MODE=true \
  -e PULUMI_POD_NETWORK="$NET" \
  -e PULUMI_POD_ADVERTISE_HOST="$ENGINE_NAME" \
  -e PULUMI_POD_ID="$POD_ID" \
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  -e STACK="$STACK" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select --create "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the Node program bootstrapped and reported its output"
if ! grep -q 'oci-node-bootstrap:' "$WORK/engine.log"; then
  echo "!! the node bootstrap shim did not run"
  exit 1
fi
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ "$GREETING" != "$EXPECTED_GREETING" ]; then
  echo "!! greeting mismatch: got '${GREETING:-<empty>}', want '$EXPECTED_GREETING'"
  exit 1
fi
echo "    greeting = $GREETING"
echo "==> language-image (Node) smoke test PASS"
