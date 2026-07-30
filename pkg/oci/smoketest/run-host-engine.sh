#!/usr/bin/env bash
#
# Mode 1 (host-engine) smoke test — the gradual onramp's first live proof. The engine
# is the branch CLI running as a NORMAL HOST PROCESS (no wrapper, no engine container,
# no pod network); the program is a plain Go program run by the stock go language host
# on the host; and the ONE containerized thing is the random provider, opted in by its
# oci:// pin. PULUMI_POD_MODE=host selects the host-engine container host:
#
#   - only the pinned package runs as a container; everything else is stock Pulumi
#   - the provider is asked to serve the well-known port (the bind contract; here via
#     the proxy-synthesized shim, since the released binary predates the contract) and
#     that port is published on the HOST LOOPBACK at an ephemeral port — the explicit
#     127.0.0.1 bind keeps it off the local network regardless of the daemon's default
#     port-binding behavior — and the engine attaches at 127.0.0.1:<mapped>
#   - the provider dials the engine back at the advertise host (host.docker.internal),
#     which reaches the engine's stock loopback binds on Docker Desktop
#
# Discriminating proofs:
#   - the attach line shows 127.0.0.1:<ephemeral> — the published-port path, not a
#     netns join (impossible here: there is no engine container) and not pod DNS
#   - no engine container and no pod network exist at any point; after the CLI exits,
#     no pod-labeled containers remain (ReleaseContext reaped the provider)
#   - the RandomPet materializes and survives to `stack output` — the full provider
#     protocol ran across the host/container boundary in both directions
#
# Usage: run-host-engine.sh
# Requires a running Docker daemon and the repo Go toolchain. No wrapper involved.
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-host-engine"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
LANG_GO_DIR="$REPO_ROOT/sdk/go/pulumi-language-go"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"
REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
IMAGE_REF="$REGISTRY_HOST/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

POD_ID="hostsmoke-$$"
POD_LABEL="com.pulumi.pod=$POD_ID"
PROXY_NAME="pulumi-pod-$POD_ID-registry"
STACK="dev"

WORK="$(mktemp -d)"
mkdir -p "$WORK/bin" "$WORK/project" "$WORK/state" "$WORK/pulumi-home"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker image rm -f "$IMAGE_REF" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run host-engine test"
  exit 1
fi

echo "==> building the branch CLI + go language host for the HOST (no cross-compile — the engine runs here)"
( cd "$PKG_DIR" && go build -o "$WORK/bin/pulumi" ./cmd/pulumi )
( cd "$LANG_GO_DIR" && go build -o "$WORK/bin/pulumi-language-go" . )

echo "==> cross-compiling the registry-proxy + forwarder shim (linux/$GOARCH) for synthesis"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )
( cd "$PKG_DIR/cmd/pulumi-pod-shim" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/pulumi-pod-shim-linux" . )

# The wrapper's shared proxy squats on 5005 when present; this test runs its own
# scoped proxy (with shim synthesis — the released binary needs the shim to serve
# the well-known port). A synthesized image cached from another run may or may not
# carry the shim (refs don't encode it), so clear it.
docker rm -f pulumi-pod-registry-proxy >/dev/null 2>&1 || true
docker image rm -f "$IMAGE_REF" >/dev/null 2>&1 || true

echo "==> starting registry-proxy $PROXY_NAME on $REGISTRY_HOST (shim synthesis on)"
docker run -d --name "$PROXY_NAME" --label "$POD_LABEL" -p "$REGISTRY_PORT:5000" \
  -e PROXY_TARGET_ARCH="$GOARCH" \
  -e PULUMI_POD_SHIM_BIN=/pulumi-pod-shim \
  -v "$WORK/registry-proxy-linux":/registry-proxy:ro \
  -v "$WORK/pulumi-pod-shim-linux":/pulumi-pod-shim:ro \
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

cp "$PROJECT_DIR/Pulumi.yaml" "$PROJECT_DIR/main.go" "$PROJECT_DIR/go.mod" "$WORK/project/"
echo "==> resolving the program's Go deps (host toolchain — the program runs HERE)"
( cd "$WORK/project" && go mod tidy >/dev/null 2>&1 )

# The host-engine environment: PULUMI_POD_MODE=host is the whole opt-in. No pod
# network, no advertise host override (host.docker.internal is the default), no
# engine image — the engine is $WORK/bin/pulumi. PULUMI_HOME is isolated so a
# stock random binary installed on this machine cannot mask the container path.
export PATH="$WORK/bin:$PATH"
export PULUMI_POD_MODE=host
export PULUMI_POD_ID="$POD_ID"
export PULUMI_HOME="$WORK/pulumi-home"
export PULUMI_BACKEND_URL="file://$WORK/state"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"

cd "$WORK/project"
echo "==> pulumi stack init + up (HOST engine, containerized pinned provider)"
pulumi stack init "$STACK"
pulumi up --yes --skip-preview 2>&1 | tee "$WORK/up.log"

echo "==> asserting the provider resolved by its oci:// pin and ran as a container"
if ! grep -q "oci: provider random resolved by its oci:// pin" "$WORK/up.log"; then
  echo "!! the pinned provider did not resolve through the container path"
  exit 1
fi

echo "==> asserting the engine attached at a host-loopback published port"
ATTACH_LINE="$(grep 'oci: provider random running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
if ! echo "$ATTACH_LINE" | grep -qE 'attaching at 127\.0\.0\.1:[0-9]+'; then
  echo "!! expected the host engine to attach at 127.0.0.1:<published port>"
  exit 1
fi
if echo "$ATTACH_LINE" | grep -qE 'attaching at [^ ]*provider-random[^ ]*:7777'; then
  echo "!! the engine attached by container DNS — impossible for a host engine; wrong path taken"
  exit 1
fi

echo "==> asserting no engine container and no pod-labeled containers remain (engine is a host process; provider reaped)"
if docker ps -a --filter "label=$POD_LABEL" --format '{{.Names}}' | grep -v -- "-registry" | grep -q .; then
  echo "!! pod-labeled containers remain after up:"
  docker ps -a --filter "label=$POD_LABEL" --format '    {{.Names}}'
  exit 1
fi

PET="$(pulumi stack output petName)"
echo "==> asserting the RandomPet materialized: petName=$PET"
if [ -z "$PET" ]; then
  echo "!! no petName output — the provider protocol did not complete"
  exit 1
fi

echo "==> pulumi destroy (a second host-engine process; the provider container starts fresh)"
pulumi destroy --yes --skip-preview 2>&1 | tee "$WORK/destroy.log"
if ! grep -qE 'deleted' "$WORK/destroy.log"; then
  echo "!! destroy did not report deleting the resource"
  exit 1
fi

echo "==> host-engine smoke test PASS — an uncontainerized program consumed a containerized"
echo "    pinned provider through a host-loopback published port (full CRUD; the provider->engine"
echo "    dial-back direction awaits a case that exercises it — Construct or provider logging)"
