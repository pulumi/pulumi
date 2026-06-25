#!/usr/bin/env bash
#
# Engine-in-container smoke test for containerized (OCI) execution — design
# Option C. Where run.sh keeps the engine in-process on the host (the program
# dials back via host.docker.internal), here the pulumi CLI *and its engine* run
# inside a container on a shared pod network, and the program container reaches
# the engine purely by container DNS. There is no path back to the host, so a
# green run proves the pod-network topology by construction.
#
# Pipeline:
#   1. cross-compile the branch's pulumi + pulumi-language-oci for linux
#   2. build an engine/CLI image from them (Dockerfile.cli)
#   3. cross-compile + build the demo program image (Dockerfile, as run.sh does)
#   4. create a pod docker network
#   5. docker run the engine image on the pod net (PULUMI_POD_MODE=true,
#      PULUMI_POD_NETWORK=<net>, docker socket + project + state mounted) running
#      `pulumi up`. Its in-container language host starts the program container on
#      the same net; the program dials <engine-name>:<port> via embedded DNS.
#   6. assert the greeting output, read inside the container against shared state
#
# Usage: run-pod.sh
#
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
# No host `pulumi` binary is needed — the engine runs in the container.
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project"
PROGRAM_DIR="$SMOKE_DIR/program"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder on this machine (e.g.
# Depot); point OCI_BUILDER at a local builder. `docker run`/`network`/`ps` are
# unaffected and use the default context.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# Pod identity. $$ keeps concurrent runs from colliding; the engine's container
# name doubles as its DNS name on the pod network.
NET="pulumi-pod-smoke-$$"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-demo:latest"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  # Force-remove anything still attached to the pod net (the engine container and
  # any straggler program container), then the network. Belt-and-suspenders: the
  # happy path already removed both via --rm. (Avoid `xargs -r`; BSD xargs on
  # macOS lacks it.)
  local stragglers
  stragglers="$(docker ps -aq --filter "network=$NET" 2>/dev/null || true)"
  [ -n "$stragglers" ] && docker rm -f $stragglers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run engine-in-container test"
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

echo "==> cross-compiling demo program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

# The CLI writes Pulumi.<stack>.yaml into its cwd; mount a copy so the repo's
# project dir stays clean. Only Pulumi.yaml is needed (pod mode runs from the
# program image, not ./program).
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME on $NET (pulumi up)"
# The engine binds 0.0.0.0 (PULUMI_POD_MODE) and advertises itself as
# $ENGINE_NAME; the language host starts the program on $NET, where that name
# resolves by embedded DNS. State lives on the mounted volume so the in-container
# output read sees what the same process just wrote.
docker run --rm -i \
  --privileged \
  --network "$NET" \
  --name "$ENGINE_NAME" \
  --hostname "$ENGINE_NAME" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$WORK/project":/project \
  -v "$WORK/state":/state \
  -w /project \
  -e PULUMI_POD_MODE=true \
  -e PULUMI_POD_NETWORK="$NET" \
  -e PULUMI_POD_ADVERTISE_HOST="$ENGINE_NAME" \
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  -e STACK="$STACK" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c 'set -e
      pulumi login "$PULUMI_BACKEND_URL"
      pulumi stack select --create "$STACK"
      pulumi up --yes --skip-preview --stack "$STACK"
      printf "SMOKE greeting=<<%s>> hostname=<<%s>>\n" \
        "$(pulumi stack output greeting --stack "$STACK")" \
        "$(pulumi stack output hostname --stack "$STACK")"' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting program ran in a container and reached the engine"
if ! grep -q 'SMOKE greeting=<<.*OCI runtime.*>>' "$WORK/engine.log"; then
  echo "!! expected greeting output containing 'OCI runtime' not found"
  exit 1
fi
echo "$(grep -o 'SMOKE greeting=.*' "$WORK/engine.log" | head -1)"
echo "==> engine-in-container smoke test PASS"
