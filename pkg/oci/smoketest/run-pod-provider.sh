#!/usr/bin/env bash
#
# Containerized-provider smoke test (design Phase 4a). Builds on run-pod.sh: the
# engine runs in a container on a pod network, and a real resource provider runs
# in *its own* container too. The key trick is the k8s pod model — the provider
# container shares the engine container's network namespace
# (`--network container:<engine>`), so the engine's existing 127.0.0.1
# spawn/attach/callback machinery is correct over the shared loopback. That means
# we can drive a STOCK released provider binary (which binds 127.0.0.1, built
# against the released SDK, not this branch) with no rebuild and no engine code:
#
#   1. download the stock provider tarball and wrap the binary in an image
#   2. engine container starts the provider in its own netns, scrapes the port
#      line from `docker logs`, and points the engine at it via
#      PULUMI_DEBUG_PROVIDERS=<pkg>:<port> — the existing attach mechanism
#   3. a program (program-random/) creates a random.RandomPet through it
#
# A green run proves the engine drove a provider-in-a-container through the full
# RegisterResource -> Create gRPC path.
#
# Usage: run-pod-provider.sh
#
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` use the default
# context and are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# The stock provider version is kept in lockstep with the SDK the program builds
# against (program-random/go.mod requires pulumi-random/sdk/v4 v4.21.0).
PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  # Backstop: remove every container this pod created (engine + provider +
  # program) by label, then the network. The inner script already tears the
  # provider down before the engine exits; this covers a mid-run failure. Filter
  # by label, not network — a netns-sharing provider is not listed under $NET.
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run containerized-provider test"
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

echo "==> running engine container $ENGINE_NAME on $NET (provider in shared netns, then pulumi up)"
# Inside the engine container: start the provider in this container's netns,
# scrape its port line, and attach to it via PULUMI_DEBUG_PROVIDERS. The provider
# binds 127.0.0.1:<port> which, in the shared netns, the engine reaches directly.
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
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  -e STACK="$STACK" \
  -e ENGINE_NAME="$ENGINE_NAME" \
  -e POD_LABEL="$POD_LABEL" \
  -e PROVIDER_PKG="$PROVIDER_PKG" \
  -e PROVIDER_IMAGE="$PROVIDER_IMAGE" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    PROVIDER_NAME="$ENGINE_NAME-provider-$PROVIDER_PKG"
    # Remove the provider before the engine exits; a netns-sharing container
    # otherwise blocks the engine container'"'"'s own --rm teardown.
    trap '"'"'docker rm -f "$PROVIDER_NAME" >/dev/null 2>&1 || true'"'"' EXIT

    echo "oci: starting $PROVIDER_PKG provider container in the engine netns"
    docker run -d --name "$PROVIDER_NAME" \
      --network "container:$ENGINE_NAME" \
      --label "$POD_LABEL" \
      "$PROVIDER_IMAGE" >/dev/null

    PORT=""
    i=0
    while [ $i -lt 30 ]; do
      PORT="$(docker logs "$PROVIDER_NAME" 2>/dev/null | grep -m1 -E "^[0-9]+$" || true)"
      [ -n "$PORT" ] && break
      i=$((i + 1)); sleep 1
    done
    if [ -z "$PORT" ]; then
      echo "!! provider did not report a port"; docker logs "$PROVIDER_NAME" || true; exit 1
    fi
    echo "oci: attaching to $PROVIDER_PKG provider at 127.0.0.1:$PORT (shared netns)"
    export PULUMI_DEBUG_PROVIDERS="$PROVIDER_PKG:$PORT"

    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select --create "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE petName=<<%s>>\n" "$(pulumi stack output petName --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the random resource was created through the containerized provider"
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the provider did not create the resource"
  exit 1
fi
echo "    petName = $PET"
echo "==> containerized-provider smoke test PASS"
