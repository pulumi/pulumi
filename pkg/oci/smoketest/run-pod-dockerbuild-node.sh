#!/usr/bin/env bash
#
# docker-build (buildkit) smoke test — the workspace-coupled provider "real prize",
# driven from a *Node* host program. This validates the one provider archetype left
# unproven: docker-build, which (unlike the classic `docker` provider) uses an
# embedded buildkit client rather than shelling out to the docker CLI. Driving it
# from Node sidesteps the docker-build Go SDK's module-consumability snag — the
# pod execution model is identical regardless of host language.
#
# The engine (pod mode) sees `docker-build` is workspace-coupled, so it runs the
# provider FROM the program image (which carries the build context at
# /workspace/app — but NO docker CLI, since buildkit is embedded), and — because
# docker-build declares the `docker-socket` capability — projects the pod's
# /var/run/docker.sock into it. The provider builds the image straight against the
# daemon's buildkit and, with `load: true`, exports it into the daemon's image
# store — so the artifact is real and inspectable, not a cache-only build.
#
# This also answers the open question from the design doc: docker-build's embedded
# buildkit drives the *same* projected docker socket we already use for the classic
# `docker` provider — no separate buildkitd endpoint needed for local execution.
#
# Pipeline:
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image (Dockerfile.cli) and the Node program image (Dockerfile.docker-build-node)
#   2. download + wrap the stock docker-build provider binary into an image
#   3. create a pod network, run `pulumi up` in the engine container
#   4. assert the provider ran workspace-coupled, got the socket, and a real image
#      landed in the daemon's store
#
# Usage: run-pod-dockerbuild-node.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-docker-build-node"
PROGRAM_DIR="$SMOKE_DIR/program-docker-build-node"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder for the images this script builds. The provider's
# own build inside the pod talks to the daemon directly over the projected socket.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# Provider version kept in lockstep with the SDK the program depends on
# (program-docker-build-node/package.json requires @pulumi/docker-build 0.0.20,
# which pins plugin docker-build 0.0.20). The engine's container host resolves the
# image by the same convention: pulumi-provider-<name>:v<version>.
PROVIDER_PKG="docker-build"
PROVIDER_VERSION="0.0.20"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-docker-build-node:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
BUILT_IMAGE="oci-pod-buildx-built:latest" # what the in-pod provider builds + loads
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
  docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run docker-build test"
  exit 1
fi

# Start from a clean slate so the post-run image-inspect proves THIS run built it,
# not a leftover from a previous run.
docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true

echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

echo "==> building engine image $ENGINE_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"

echo "==> building Node program image $PROGRAM_IMAGE (npm install @pulumi/pulumi + @pulumi/docker-build, bakes /workspace/app)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile.docker-build-node" "$PROGRAM_DIR"

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

echo "==> running engine container $ENGINE_NAME on $NET (engine runs docker-build FROM the program image, socket projected)"
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
    printf "SMOKE ref=<<%s>>\n" "$(pulumi stack output ref --stack "$STACK")"
    printf "SMOKE digest=<<%s>>\n" "$(pulumi stack output digest --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the docker-build provider ran from the program image, got the socket, and built a real image"
if ! grep -q "oci: provider $PROVIDER_PKG is workspace-coupled" "$WORK/engine.log"; then
  echo "!! the engine did not run docker-build from the program image"
  exit 1
fi
if ! grep -q "oci: provider $PROVIDER_PKG gets capability \"docker-socket\"" "$WORK/engine.log"; then
  echo "!! the docker socket capability was not projected"
  exit 1
fi

REF="$(sed -n 's/.*SMOKE ref=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$REF" ]; then
  echo "!! ref output empty — the build did not produce an image ref"
  exit 1
fi
echo "    ref    = $REF"
echo "    digest = $(sed -n 's/.*SMOKE digest=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"

# The decisive proof: the provider exported the image (load: true) into the
# projected daemon — which is the host daemon — so a real, inspectable artifact
# now exists in the host's image store. This is what an empty-export build (the
# spike's contextHash-only state) could NOT produce.
echo "==> asserting the built image is a real artifact in the daemon's store"
if ! docker image inspect "$BUILT_IMAGE" >/dev/null 2>&1; then
  echo "!! $BUILT_IMAGE is not in the daemon — load: true did not export a real image"
  exit 1
fi
echo "    $BUILT_IMAGE present in daemon ($(docker image inspect -f '{{.Id}}' "$BUILT_IMAGE"))"
echo "==> docker-build (buildkit) smoke test PASS — embedded buildkit drove the projected socket"
