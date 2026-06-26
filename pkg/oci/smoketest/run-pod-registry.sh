#!/usr/bin/env bash
#
# OCI-registry plugin install smoke test (the north star: distribute and install
# provider plugins as images via OCI registry infrastructure). Unlike
# run-pod-provider.sh — which wraps the provider image and leaves it sitting in the
# local daemon for the engine to find — here the image lives ONLY in a registry.
# The engine must *install* it: on first use the container host sees the image is
# absent, pulls it from the registry, and runs it — all under one registry-
# qualified name (PULUMI_POD_PLUGIN_REGISTRY threads through providerImageRef, so
# resolution, pull, and run share the same name; no retag). This is the
# container-model analogue of downloading a plugin binary, and it completes the
# install path that was previously only "run it if already present".
#
# Pipeline:
#   1. build the engine + program images (as usual)
#   2. wrap the stock random provider into pulumi-provider-random:v4.21.0
#   3. stand up a local registry, push the image to it, then DELETE it from the
#      local daemon — so it exists only in the registry
#   4. run `pulumi up` with PULUMI_POD_PLUGIN_REGISTRY set; the engine pulls the
#      provider image from the registry on first use and creates the resource
#   5. assert the install-by-pull happened and the image is now present locally
#
# Usage: run-pod-registry.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION" # the bare convention ref
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

# Local OCI registry the provider image is published to and installed from. Port
# 5005 avoids the macOS-AirPlay-on-5000 clash. The engine pulls via the mounted
# docker socket, so the *daemon* reaches localhost:5005 (its own published port).
REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
REGISTRY_NAME="$NET-registry"
REGISTRY_REF="$REGISTRY_HOST/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$PROVIDER_IMAGE" "$REGISTRY_REF" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run registry test"
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

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION and wrapping it"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

echo "==> starting local OCI registry $REGISTRY_NAME on $REGISTRY_HOST"
docker run -d --name "$REGISTRY_NAME" --label "$POD_LABEL" -p "$REGISTRY_PORT:5000" registry:2 >/dev/null
for _ in $(seq 1 30); do
  curl -sf "http://$REGISTRY_HOST/v2/" >/dev/null 2>&1 && break
  sleep 0.5
done

echo "==> publishing $PROVIDER_IMAGE to the registry, then deleting it locally"
docker tag "$PROVIDER_IMAGE" "$REGISTRY_REF"
docker push "$REGISTRY_REF" >/dev/null
# Delete both tags so the provider image exists ONLY in the registry — the engine
# must install it by pulling, not find a leftover local copy.
docker image rm -f "$PROVIDER_IMAGE" "$REGISTRY_REF" >/dev/null 2>&1 || true
if docker image inspect "$PROVIDER_IMAGE" >/dev/null 2>&1; then
  echo "!! $PROVIDER_IMAGE is still present locally — test setup is invalid"
  exit 1
fi
echo "    $PROVIDER_IMAGE now exists only in $REGISTRY_HOST"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME (engine installs the provider by pulling its image)"
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
  -e PULUMI_POD_PLUGIN_REGISTRY="$REGISTRY_HOST" \
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

echo "==> asserting the engine installed the provider by pulling its registry-qualified image"
if ! grep -q "oci: installed plugin $REGISTRY_REF by pulling its image" "$WORK/engine.log"; then
  echo "!! the engine did not install the provider by pulling from the registry"
  exit 1
fi
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the pulled provider did not create the resource"
  exit 1
fi
echo "    petName = $PET"

# Resolution, pull, and run all used the single qualified name (no retag), so the
# qualified ref is now present locally.
echo "==> asserting the pulled image is present under its qualified name"
if ! docker image inspect "$REGISTRY_REF" >/dev/null 2>&1; then
  echo "!! $REGISTRY_REF is not present after the run — the pull did not land it"
  exit 1
fi
echo "    $REGISTRY_REF present ($(docker image inspect -f '{{.Id}}' "$REGISTRY_REF"))"
echo "==> OCI-registry plugin install smoke test PASS — a provider plugin was installed by pulling its qualified image"
