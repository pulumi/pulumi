#!/usr/bin/env bash
#
# package-add-from-image smoke test (the dev/package-time half of the OCI model).
# Proves `pulumi package add oci://<ref>` end-to-end: a provider is consumed as an
# IMAGE, not a binary at a filesystem path. Two phases, deliberately split so a failure
# localizes to one of three first-time integrations rather than a single opaque red:
#
#   Phase 1 — schema-from-image (no codegen, no delegate, no loader):
#     `pulumi package get-schema oci://<proxy>/pulumi-provider-random:vX` runs the
#     provider image as a one-shot pod container, attaches over the shared engine netns,
#     and reads GetSchema. The proxy synthesizes the image from the released binary on
#     pull. This isolates the novel pod/attach wiring (oci.ProviderFromImage).
#
#   Phase 2 — package add (layers codegen + delegation + loader onto a proven phase 1):
#     `pulumi package add oci://<proxy>/...` in an oci project (runtime.options.language:
#     go) generates a local SDK and records the package. The schema comes from the image
#     (phase 1); the OCI language host then DELEGATES SDK *generation* (GeneratePackage) to
#     pulumi-language-go (the dev-language axis), and runs the BUILD-OWNED link command in
#     the build environment image (build.{image,link}) — linking is package-manager-specific,
#     so the build owns it, not the language host. Asserts a Go SDK lands in sdks/, the
#     build.link command ran in the toolchain image, and the oci:// REF — not a path — is
#     written into Pulumi.yaml.
#
# Together: a package is a plugin image; the consumer needs no provider toolchain (the
# image carries it), and the engine only ever sees a ref. The delegate is `go` here
# because the delegation mechanism is language-agnostic (proven hermetically in the unit
# tests) and a static Go host runs on the alpine engine image with no extra toolchain.
#
# The oci:// ref is fully qualified (it carries the proxy host); PULUMI_POD_PLUGIN_REGISTRY
# only gates that an absent image MAY be pulled. The image pull rides the mounted docker
# socket to the host daemon, which resolves localhost:5005 (the proxy's published port).
#
# Usage: run-pod-package-add.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-package-add"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
# pulumi-language-go is its own Go module.
LANG_GO_DIR="$REPO_ROOT/sdk/go/pulumi-language-go"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
# A codegen-capable engine image: the base engine image plus a real delegate language
# host (pulumi-language-go) the OCI host forwards SDK generation to.
ENGINE_IMAGE_CODEGEN="pulumi-cli-oci-codegen:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"

REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
PROXY_NAME="$NET-registry"
IMAGE_REF="$REGISTRY_HOST/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
OCI_SOURCE="oci://$IMAGE_REF"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project" "$WORK/state"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$IMAGE_REF" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run package-add test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling delegate language host pulumi-language-go (linux/$GOARCH)"
( cd "$LANG_GO_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-go-linux" . )

echo "==> building codegen engine image $ENGINE_IMAGE_CODEGEN (base + pulumi-language-go)"
cat > "$WORK/cli/Dockerfile.codegen" <<EOF
FROM $ENGINE_IMAGE
COPY pulumi-language-go-linux /usr/local/bin/pulumi-language-go
EOF
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE_CODEGEN" -f "$WORK/cli/Dockerfile.codegen" "$WORK/cli"

echo "==> cross-compiling the registry-proxy (linux/$GOARCH)"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

echo "==> starting registry-proxy $PROXY_NAME on $REGISTRY_HOST (synthesizes provider images on pull)"
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

# The shared docker args for running a CLI command in the engine container, in pod mode.
# package add/get-schema do dev-time work (no deployment), but ProviderFromImage still
# needs the pod contract: the docker socket (to start the schema container), the pod
# network + advertise host (the schema container joins this engine's netns), and the
# plugin registry (to allow pulling the absent image through the proxy).
engine_run() {
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
    --entrypoint sh \
    "$ENGINE_IMAGE_CODEGEN" \
    -c "$1"
}

###############################################################################
# Phase 1: schema-from-image (isolates oci.ProviderFromImage — run image, attach,
# GetSchema). No project, no codegen, no delegate.
###############################################################################
echo "==> PHASE 1: pulumi package get-schema $OCI_SOURCE (run the image, read its schema)"
engine_run "pulumi package get-schema '$OCI_SOURCE'" 2>&1 | tee "$WORK/get-schema.log"

echo "==> asserting the proxy synthesized the provider image on demand (nothing pre-built)"
if ! docker logs "$PROXY_NAME" 2>&1 | grep -q "synthesizing pulumi-provider-$PROVIDER_PKG"; then
  echo "!! the proxy did not synthesize the provider image — schema fetch did not pull through it"
  docker logs "$PROXY_NAME" 2>&1 | tail -20
  exit 1
fi
echo "    proxy synthesized pulumi-provider-$PROVIDER_PKG from the released binary"

echo "==> asserting the schema is the random provider's (read from the running image)"
if ! grep -q '"name": "random"' "$WORK/get-schema.log"; then
  echo "!! get-schema did not return the random provider's schema"
  exit 1
fi
if ! grep -qE 'Random(String|Uuid|Pet|Id|Integer)' "$WORK/get-schema.log"; then
  echo "!! the schema is missing the expected random provider resources"
  exit 1
fi
echo "    schema-from-image works: random's schema came out of the running container"

# The schema fetch stops ONLY its own container (StopContainer), never the whole pod —
# the engine/proxy must outlive it. Assert the schema container is gone (the cleanup trap
# would otherwise mask a leak) while the proxy is still up (phase 2 relies on it).
echo "==> asserting the schema container was stopped (per-container stop, not pod-wide cleanup)"
if docker ps --filter "label=$POD_LABEL" --format '{{.Names}}' | grep -q -- '-schema-'; then
  echo "!! a schema container is still running — ProviderFromImage leaked it"
  docker ps --filter "label=$POD_LABEL" --format '    {{.Names}}'
  exit 1
fi
echo "    schema container stopped; proxy + network still up"

###############################################################################
# Phase 2: package add (layers codegen delegation + loader onto a proven phase 1).
###############################################################################
# An oci project whose dev-language is go has a Go program, hence a go.mod.
cp "$PROJECT_DIR/Pulumi.yaml" "$PROJECT_DIR/go.mod" "$WORK/project/"

echo "==> PHASE 2: pulumi package add $OCI_SOURCE (delegate SDK gen to pulumi-language-go; build-owned link)"
engine_run "pulumi package add '$OCI_SOURCE'" 2>&1 | tee "$WORK/add.log"

echo "==> asserting a Go SDK was generated locally (codegen delegated to pulumi-language-go)"
if ! find "$WORK/project/sdks" -name '*.go' 2>/dev/null | grep -q .; then
  echo "!! no Go SDK files were generated under sdks/ — delegation or codegen failed"
  find "$WORK/project" -type f 2>/dev/null | sed 's/^/    /' | head -40
  exit 1
fi
SDK_FILE="$(find "$WORK/project/sdks" -name '*.go' | head -1)"
echo "    Go SDK generated: ${SDK_FILE#$WORK/project/}"

echo "==> asserting the oci:// REF (not a path) was recorded in Pulumi.yaml"
if ! grep -q "$OCI_SOURCE" "$WORK/project/Pulumi.yaml"; then
  echo "!! the oci:// ref was not written into Pulumi.yaml's packages"
  echo "--- Pulumi.yaml ---"; cat "$WORK/project/Pulumi.yaml"
  exit 1
fi
echo "    Pulumi.yaml records the package as: $OCI_SOURCE"

echo "==> asserting Link was BUILD-OWNED (the build.link command ran in the toolchain image)"
RECEIPT="$WORK/project/LINK_RECEIPT"
if [ ! -f "$RECEIPT" ]; then
  echo "!! no LINK_RECEIPT — the build.link command did not run (linking is build-owned, not delegated)"
  exit 1
fi
# Discriminating: `go version` only succeeds in the golang build image — the alpine engine
# has no go toolchain, so had linking run in-host (or in the language plugin) this would be absent.
if ! grep -q 'go version go' "$RECEIPT"; then
  echo "!! LINK_RECEIPT lacks a go toolchain version — link did not run in the build environment"
  cat "$RECEIPT"
  exit 1
fi
# Proves the OCI host handed the generated SDK path to the build.link command.
if ! grep -q 'sdk: sdks/random' "$RECEIPT"; then
  echo "!! LINK_RECEIPT lacks the SDK path — the link command did not receive PULUMI_LINK_SDK_PATHS"
  cat "$RECEIPT"
  exit 1
fi
echo "    build-owned link ran in golang:alpine and received the SDK path ($(grep '^sdk:' "$RECEIPT"))"

echo "==> package-add-from-image smoke test PASS — a provider was consumed as an IMAGE:"
echo "    schema read from the running container, SDK generated via delegation, ref (not path) recorded"
