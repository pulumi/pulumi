#!/usr/bin/env bash
#
# Image-build smoke test (design Phase 5, the workspace-coupled "real prize").
# Proves a provider that both needs the program's filesystem AND a host capability
# works in the pod model: the `docker` provider builds an image from a context
# baked into the program image, reaching the daemon over a projected docker socket.
#
# The engine (pod mode) sees `docker` is workspace-coupled, so it runs the docker
# provider FROM the program image (which carries the build context + docker CLI),
# and — because docker declares the `docker-socket` capability — mounts the pod's
# /var/run/docker.sock into it. The provider runs `docker build /workspace/app`
# against that socket. None of the build context, the CLI, or the socket lives in
# the provider's own image: context + CLI come from the program image, the socket
# from the pod.
#
# The classic `docker` provider is used rather than `docker-build` only because the
# latter's Go SDK isn't cleanly consumable via go modules right now; the pod
# execution model is identical.
#
# Usage: run-pod-dockerbuild.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-docker-build"
PROGRAM_DIR="$SMOKE_DIR/program-docker-build"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder for the images this script builds. The provider's
# own build inside the pod talks to the daemon directly over the projected socket.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# Provider version kept in lockstep with the SDK the program builds against
# (program-docker-build/go.mod requires pulumi-docker/sdk/v4 v4.11.2).
PROVIDER_PKG="docker"
PROVIDER_VERSION="4.11.2"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-docker:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  local vols
  vols="$(docker volume ls -q --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$vols" ] && docker volume rm -f $vols >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f oci-pod-built:latest >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run image-build test"
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

echo "==> building program image $PROGRAM_IMAGE (bakes /workspace/app + docker CLI)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile.docker" "$SMOKE_DIR"

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH) and wrapping it"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME on $NET (engine runs docker FROM the program image, socket projected)"
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
  -e PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE" \
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
    printf "SMOKE imageName=<<%s>>\n" "$(pulumi stack output imageName --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the docker provider ran from the program image, got the socket, and built the image"
if ! grep -q 'oci: provider docker is workspace-coupled' "$WORK/engine.log"; then
  echo "!! the engine did not run docker from the program image"
  exit 1
fi
if ! grep -q 'oci: provider docker gets capability "docker-socket"' "$WORK/engine.log"; then
  echo "!! the docker socket capability was not projected"
  exit 1
fi
IMG="$(sed -n 's/.*SMOKE imageName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
case "$IMG" in
  *oci-pod-built*) echo "    imageName = $IMG" ;;
  *) echo "!! imageName output missing/unexpected: '${IMG:-<empty>}' — the build did not complete"; exit 1 ;;
esac
echo "==> image-build smoke test PASS"
