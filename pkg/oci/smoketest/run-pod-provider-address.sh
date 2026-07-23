#!/usr/bin/env bash
#
# Address-model provider smoke test (the minimal mechanism proof for the forwarder
# shim + attach-by-DNS). Same shape as run-pod-provider.sh, but the provider runs as
# its OWN container on the pod network — not in the engine's netns — and the engine
# attaches to it by container DNS name at the shim's well-known port, rather than over
# 127.0.0.1. This exercises the whole address path with a single provider and one
# image, before the multi-provider case adds registry + collision moving parts.
#
# What differs from run-pod-provider.sh:
#   - the provider image embeds the forwarder shim (Dockerfile.provider-shim), so the
#     stock loopback-binding binary is reachable at 0.0.0.0:7777;
#   - the engine runs with PULUMI_POD_ADDRESS_MODE=true, so pkg/oci puts the provider
#     on the pod network and attaches at <container-dns>:7777 (+ rewrites the engine
#     address it hands back to the advertised DNS name for the reverse direction).
#
# Usage: run-pod-provider-address.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random-single" # one RandomPet via the default provider
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
# Tag the local image with the public-source-qualified name the default resolver computes
# (DefaultPublicRegistry/pulumi/pulumi-provider-<name>), so it is a local store hit and the
# engine never tries to pull it — no registry proxy needed for this minimal test.
PROVIDER_IMAGE="pulumi.registry.internal/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  # Remove every container labeled for this pod — the engine and the provider it
  # started. In address mode the provider is a normal member of $NET (not netns-joined),
  # so it is also caught by the network teardown; the label sweep covers a mid-run failure.
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run address-model provider test"
  exit 1
fi

build_engine_image # also cross-compiles the shim to $WORK/cli/pulumi-pod-shim-linux

echo "==> cross-compiling demo program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH) and wrapping it WITH the shim"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
cp "$WORK/cli/pulumi-pod-shim-linux" "$WORK/provctx/pulumi-pod-shim"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider-shim" "$WORK/provctx"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME on $NET (address mode: provider joins the pod net, attached by DNS)"
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
  -e PULUMI_POD_ADDRESS_MODE=true \
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
    printf "SMOKE petName=<<%s>>\n" "$(pulumi stack output petName --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the provider was attached by DNS address (not 127.0.0.1)"
if ! grep -q 'oci: provider random running as container' "$WORK/engine.log"; then
  echo "!! the engine did not start the provider as a container"; exit 1
fi
# In address mode the attach line names the container DNS host and the shim port, e.g.
# "attaching at provider-random-xxxx:7777" — assert it is NOT a loopback attach.
if grep -q 'attaching at 127.0.0.1' "$WORK/engine.log"; then
  echo "!! provider was attached over 127.0.0.1 — address mode did not take effect"; exit 1
fi
if ! grep -qE 'attaching at [^ ]+:7777' "$WORK/engine.log"; then
  echo "!! did not see an attach at the shim's well-known port :7777"; exit 1
fi
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the provider did not create the resource"; exit 1
fi
echo "    petName = $PET"
echo "==> address-model provider smoke test PASS"
