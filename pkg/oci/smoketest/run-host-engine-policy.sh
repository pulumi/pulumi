#!/usr/bin/env bash
#
# Mode 1 policy smoke test — a HOST engine drives a CONTAINERIZED policy pack. The
# program is the plain host-run Go program from run-host-engine.sh (pinned random
# provider through the proxy); the pack is the TypeScript oci-policy-smoke pack,
# pinned by ref (`--policy-pack oci://<ref>`, image tagged locally so the store
# check hits). Host mode is address mode by construction, so the pack's analyzer
# must honor PULUMI_PLUGIN_LISTEN_ADDRESS — the bind-contract @pulumi/policy is
# staged from a local pulumi-policy clone (REQUIRED here: there is no shim on the
# policy path and no netns to fall back to). The engine attaches the pack at a
# host-loopback published port; the pack's dial-back target (PULUMI_ENGINE) is the
# advertised host.docker.internal.
#
# Discriminating proofs:
#   - the pack's attach line shows 127.0.0.1:<published> (host publish path)
#   - the violation message carries the marker baked into the POLICY image alone,
#     read inside validateResource — the policy logic ran from its own container
#     while the engine and program ran on the host
#
# Usage: run-host-engine-policy.sh
# Requires a running Docker daemon, the repo Go toolchain, and a pulumi-policy
# clone (OCI_POLICY_SDK_DIR overrides the default ~/src/pulumi/pulumi-policy).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-host-engine"
POLICY_DIR="$SMOKE_DIR/policy-pack-node"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
LANG_GO_DIR="$REPO_ROOT/sdk/go/pulumi-language-go"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
PROVIDER_REF="$REGISTRY_HOST/pulumi/pulumi-provider-random:v4.21.0"
# The pack's pin ref; the built image is tagged exactly this (store hit, no pull).
POLICY_REF="$REGISTRY_HOST/pulumi/pulumi-policy-smoke:v1.0.0"
# A throwaway scratch image carrying just the bind-contract @pulumi/policy, so the pack image
# can overlay it via the same --from mechanism run-pod-policy uses (host mode has no engine
# image to bake it into, so we mint one here).
POLICY_SDK_IMAGE="oci-smoke-policy-sdk:latest"
EXPECTED_MARKER="oci-policy-ran-from-its-own-image"

POD_ID="hostpolicy-$$"
POD_LABEL="com.pulumi.pod=$POD_ID"
PROXY_NAME="pulumi-pod-$POD_ID-registry"
STACK="dev"

BUILDER="${OCI_BUILDER:-desktop-linux}"

# See run-host-engine.sh: bind-mount sources must be daemon-visible; colima
# shares only ~ and /tmp/colima, not the mktemp default under /var/folders.
if [ -n "${OCI_SMOKE_TMPDIR:-}" ]; then
  mkdir -p "$OCI_SMOKE_TMPDIR"
  WORK="$(mktemp -d "$OCI_SMOKE_TMPDIR/oci-smoke.XXXXXX")"
else
  WORK="$(mktemp -d)"
fi
mkdir -p "$WORK/bin" "$WORK/project" "$WORK/state" "$WORK/pulumi-home"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker image rm -f "$PROVIDER_REF" "$POLICY_REF" "$POLICY_SDK_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$POLICY_DIR/policy-sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run host-engine policy test"
  exit 1
fi

echo "==> building the branch CLI + go language host for the HOST"
( cd "$PKG_DIR" && go build -o "$WORK/bin/pulumi" ./cmd/pulumi )
( cd "$LANG_GO_DIR" && go build -o "$WORK/bin/pulumi-language-go" . )

# ── stage the bind-contract @pulumi/policy (REQUIRED in host mode) ────────────
POLICY_SDK_DIR="${OCI_POLICY_SDK_DIR:-$HOME/src/pulumi/pulumi-policy}/sdk/nodejs/policy"
if [ ! -d "$POLICY_SDK_DIR" ]; then
  echo "!! host mode needs the bind-contract @pulumi/policy and no clone was found at $POLICY_SDK_DIR"
  echo "   (git get pulumi/pulumi-policy, or point OCI_POLICY_SDK_DIR at a clone)"
  exit 1
fi
echo "==> building the bind-contract @pulumi/policy ($POLICY_SDK_DIR -> bin/)"
(cd "$POLICY_SDK_DIR" && bun install >/dev/null && bun run tsc >/dev/null)
if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$POLICY_SDK_DIR/bin/server.js"; then
  echo "!! the built policy SDK's serve site does not honor the bind contract —"
  echo "   bin/ is stale or the clone lacks the patch"
  exit 1
fi
rm -rf "$POLICY_DIR/policy-sdk-bin"
mkdir -p "$POLICY_DIR/policy-sdk-bin"
cp -R "$POLICY_SDK_DIR/bin/." "$POLICY_DIR/policy-sdk-bin/"
rm -f "$POLICY_DIR/policy-sdk-bin/package.json" # overlay is code, not identity

# The shared policy Dockerfile overlays the SDK from an image (--from), the same as the skill's
# template will from the published pod image. Host mode has no engine image, so bake the staged
# bin into a throwaway scratch image and point the build at it.
echo "==> baking the bind-contract @pulumi/policy into $POLICY_SDK_IMAGE (overlay source)"
printf 'FROM scratch\nCOPY policy-sdk-bin/ /opt/pulumi-sdk/policy/\n' | \
  docker buildx build --builder "$BUILDER" --load -t "$POLICY_SDK_IMAGE" -f - "$POLICY_DIR" >/dev/null

echo "==> building the policy image and tagging it as the pin ref $POLICY_REF"
docker buildx build --builder "$BUILDER" --load \
  --build-arg PULUMI_SDK_IMAGE="$POLICY_SDK_IMAGE" \
  -t "$POLICY_REF" -f "$POLICY_DIR/Dockerfile" "$POLICY_DIR"

echo "==> cross-compiling the registry-proxy + forwarder shim (linux/$GOARCH) for the random pull"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )
( cd "$PKG_DIR/cmd/pulumi-pod-shim" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/pulumi-pod-shim-linux" . )

docker rm -f pulumi-pod-registry-proxy >/dev/null 2>&1 || true
docker image rm -f "$PROVIDER_REF" >/dev/null 2>&1 || true

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
echo "==> resolving the program's Go deps (host toolchain)"
( cd "$WORK/project" && go mod tidy >/dev/null 2>&1 )

export PATH="$WORK/bin:$PATH"
export PULUMI_POD_MODE=host
export PULUMI_POD_ID="$POD_ID"
export PULUMI_HOME="$WORK/pulumi-home"
export PULUMI_BACKEND_URL="file://$WORK/state"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"

cd "$WORK/project"
echo "==> pulumi stack init + up --policy-pack oci://$POLICY_REF (host engine, containerized pack)"
pulumi stack init "$STACK"
pulumi up --yes --skip-preview --policy-pack "oci://$POLICY_REF" 2>&1 | tee "$WORK/up.log"

echo "==> asserting the pack resolved by its pin and attached at a host-loopback published port"
if ! grep -q "oci: policy pack .* resolved by its oci:// pin" "$WORK/up.log"; then
  echo "!! the pinned policy pack did not resolve through the container path"
  exit 1
fi
ATTACH_LINE="$(grep 'oci: policy pack .* running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
if ! echo "$ATTACH_LINE" | grep -qE 'attaching at 127\.0\.0\.1:[0-9]+'; then
  echo "!! expected the host engine to attach the pack at 127.0.0.1:<published port>"
  exit 1
fi

echo "==> asserting the policy ran from its own image (violation carries the baked marker)"
if ! grep -q "marker=$EXPECTED_MARKER" "$WORK/up.log"; then
  echo "!! expected policy violation with marker=$EXPECTED_MARKER not found"
  echo "   (the policy did not run from its image, or never evaluated the RandomPet)"
  exit 1
fi
echo "    found violation with marker=$EXPECTED_MARKER"

echo "==> pulumi destroy"
pulumi destroy --yes --skip-preview 2>&1 | tee "$WORK/destroy.log"
grep -qE 'deleted' "$WORK/destroy.log" || { echo "!! destroy did not report deleting"; exit 1; }

echo "==> host-engine policy smoke test PASS — a host engine drove a containerized policy"
echo "    pack through a published port; the pack's toolchain and logic lived in its image"
