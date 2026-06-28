#!/usr/bin/env bash
#
# Registry-proxy plugin install smoke test (the force multiplier: stop pre-building
# provider images entirely). This is run-pod-registry.sh with the manual wrapping
# removed. There, the test downloaded the stock random provider binary, wrote a
# Dockerfile.provider around it, `docker build`+`docker push`ed it to a registry:2,
# and deleted it locally — all so the engine could install it by pulling. Here a
# pull-through registry-proxy does that on the fly: on the engine's pull it fetches
# the released binary from get.pulumi.com and synthesizes the same /plugin/provider
# image in memory. Nothing is pre-built or pushed; the proxy conjures the image the
# moment the daemon asks for it.
#
# The diff against run-pod-registry.sh IS the proof: the same install-by-pull test
# (same PULUMI_POD_PLUGIN_REGISTRY plumbing, same engine pull path, same resource
# created) passes with the download/wrap/build/push/delete steps gone. So the proxy
# provably subsumes the hand-wrapping — which is what lets every released provider
# be available as an image without anyone re-publishing it, and what future smoke
# tests (and `package add` schema extraction) can point at instead of pre-building.
#
# Pipeline:
#   1. build the engine + program images (the pod infra — still this branch's CLI)
#   2. cross-compile the registry-proxy and run it as the pod's registry on :5005
#   3. run `pulumi up` with PULUMI_POD_PLUGIN_REGISTRY set; the engine pulls
#      pulumi-provider-random:v4.21.0 from the proxy, which synthesizes it from the
#      released binary; the engine runs it and creates the resource
#   4. assert install-by-pull happened, the proxy synthesized the image, and the
#      resource was created
#
# Usage: run-pod-proxy.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
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
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

# The registry-proxy stands in for the registry. Same localhost:5005 reachability
# as registry:2 (a published container port the daemon reaches over its own
# loopback), so PULUMI_POD_PLUGIN_REGISTRY plumbing is unchanged. The engine
# resolves the bare convention ref to this qualified one for resolve/pull/run.
REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
PROXY_NAME="$NET-registry"
REGISTRY_REF="$REGISTRY_HOST/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$REGISTRY_REF" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run registry-proxy test"
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

echo "==> cross-compiling the registry-proxy (linux/$GOARCH)"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )

echo "==> starting registry-proxy $PROXY_NAME on $REGISTRY_HOST (synthesizes provider images on pull)"
# alpine + ca-certificates so the proxy can fetch released binaries over HTTPS; the
# proxy binary is bind-mounted in (no image to build). Publishes :5000 as :5005.
docker run -d --name "$PROXY_NAME" --label "$POD_LABEL" -p "$REGISTRY_PORT:5000" \
  -e PROXY_TARGET_ARCH="$GOARCH" \
  -v "$WORK/registry-proxy-linux":/registry-proxy:ro \
  alpine sh -c 'apk add --no-cache ca-certificates >/dev/null 2>&1 && exec /registry-proxy' >/dev/null
for _ in $(seq 1 30); do
  curl -sf "http://$REGISTRY_HOST/v2/" >/dev/null 2>&1 && break
  sleep 0.5
done
if ! curl -sf "http://$REGISTRY_HOST/v2/" >/dev/null 2>&1; then
  echo "!! registry-proxy did not come up on $REGISTRY_HOST"
  docker logs "$PROXY_NAME" 2>&1 | tail -20
  exit 1
fi
echo "    registry-proxy up; NO provider image was pre-built"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME (engine installs the provider by pulling from the proxy)"
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
  echo "!! the engine did not install the provider by pulling from the proxy"
  exit 1
fi

echo "==> asserting the proxy synthesized the provider image on demand (nothing was pre-built)"
if ! docker logs "$PROXY_NAME" 2>&1 | grep -q "synthesizing pulumi-provider-$PROVIDER_PKG"; then
  echo "!! the proxy did not synthesize the provider image — install did not go through it"
  docker logs "$PROXY_NAME" 2>&1 | tail -20
  exit 1
fi
echo "    proxy synthesized pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION from the released binary"

PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the synthesized provider did not create the resource"
  exit 1
fi
echo "    petName = $PET"

echo "==> asserting the pulled (synthesized) image is present under its qualified name"
if ! docker image inspect "$REGISTRY_REF" >/dev/null 2>&1; then
  echo "!! $REGISTRY_REF is not present after the run — the pull did not land it"
  exit 1
fi
echo "    $REGISTRY_REF present ($(docker image inspect -f '{{.Id}}' "$REGISTRY_REF"))"
echo "==> registry-proxy smoke test PASS — a provider was installed as an image with NOTHING pre-built; the proxy wrapped the released binary on pull"
