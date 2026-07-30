#!/usr/bin/env bash
#
# Run-from-program-image provider smoke test (design Phase 5). Exercises the
# run-from-program-image model: a provider that needs the program's ambient
# toolchain (here `command`) runs *from the program image* with its binary
# injected, rather than from its own image as every other provider does.
#
# It also proves environment projection: the engine forwards a stand-in credential
# (OCI_SMOKE_FAKE_CRED) that the program image lacks, the container host projects
# the engine's environment onto the provider container, and the program reads the
# credential back — the path a real cloud provider's credentials would take.
#
# The program uses command.local.Command to run `jq` — a binary baked onto the PROGRAM
# image's PATH and present in no provider image. That is the control for placement: the
# shared workspace mount carries files, not a toolchain, so only a provider running from
# the program image can find jq. The engine (in pod mode) sees `command` runs from the
# program image, copies the stock command binary out of its provider image into an
# ephemeral volume, and runs it from the program image (PULUMI_POD_PROGRAM_IMAGE) on the
# engine's netns, then attaches.
#
# It also reads /workspace/marker, but that is only a workspace check — every provider
# mounts the volume the program image seeds, so any of them would read it too.
#
# Pipeline:
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image (Dockerfile.cli) and the demo program image (Dockerfile.command,
#      which bakes jq onto PATH and a marker into /workspace)
#   2. download + wrap the stock command provider binary into an image
#   3. create a pod network, run `pulumi up` in the engine container
#   4. the engine runs the command provider FROM the program image; it finds jq on
#      the program's PATH and reads the baked marker
#
# ADDRESS MODE (OCI_ADDRESS_MODE=1) runs the same proof over the address model. The
# command provider is the injected-binary archetype: a released binary that predates
# the SDK bind contract, exec'd from the PROGRAM image. Reachability comes from the
# injected-shim exec — the wrapper's proxy synthesizes the provider image with
# /plugin/shim baked in, the engine's injection copies the whole dir, and the
# entrypoint boots the shim when (and only when) the copy carried it — so the engine
# attaches by container DNS at :7777 while the provider still runs rooted in the
# program image.
#
# Usage: run-pod-command.sh                    # netns default (shared loopback)
#        OCI_ADDRESS_MODE=1 run-pod-command.sh # address mode (own netns, DNS:7777)
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-command"
PROGRAM_DIR="$SMOKE_DIR/program-command"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` use the default
# context and are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# The stock provider version is kept in lockstep with the SDK the program builds
# against (program-command/go.mod requires pulumi-command/sdk v1.1.0). The engine's
# container host resolves the image by the same convention:
# pulumi/pulumi-provider-<name>:v<version>.
PROVIDER_PKG="command"
PROVIDER_VERSION="1.1.0"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-command:latest"
PROVIDER_IMAGE="pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
STACK="dev"
EXPECTED_TOOLCHAIN="toolchain-from-the-program-image"
EXPECTED_MARKER="hello-from-the-program-workspace"
EXPECTED_CRED="fake-cloud-credential-9f3a"

ADDRESS_MODE="${OCI_ADDRESS_MODE:-}"
MODE_LABEL="netns (provider shares the engine netns, dialed over 127.0.0.1)"
if [ -n "$ADDRESS_MODE" ]; then
  MODE_LABEL="address (provider in its own netns, injected shim serving DNS:7777)"
fi

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the cross-compiled binary and the scratch dir.
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run run-from-program-image provider test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling demo program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE (bakes jq onto PATH + /workspace/marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile.command" "$SMOKE_DIR"

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH) and wrapping it"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Drive the deployment with the wrapper — it bootstraps the pod (network, engine
# container, PULUMI_POD_* contract, mounts, teardown) and defaults the backend +
# stack state into the mounted dir. PULUMI_POD_PROGRAM_IMAGE is forwarded so the
# command provider runs from the program image.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"
if [ -n "$ADDRESS_MODE" ]; then
  export PULUMI_POD_ADDRESS_MODE=1 # forwarded host->engine by the wrapper's env projection
else
  # The wrapper defaults address mode ON; the netns run must pin the legacy mode
  # explicitly (empty = netns, per the wrapper contract) or it silently tests the
  # wrong topology.
  export PULUMI_POD_ADDRESS_MODE=
fi

# A stand-in credential the engine has but the program image does not. The wrapper
# forwards the whole host env into the engine by default (no naming needed), and the
# container host then projects the engine's environment onto the command provider's
# container, where the program reads it back — the path a real cloud provider's
# credentials would take.
export OCI_SMOKE_FAKE_CRED="$EXPECTED_CRED"

echo "==> pulumi-pod [$MODE_LABEL]: stack init + up + output (engine runs command FROM the program image)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"
TOOLCHAIN="$("$WRAPPER" stack output toolchain)"
MARKER="$("$WRAPPER" stack output marker)"

echo "==> asserting the command provider ran from the program image"
if ! grep -q "oci: provider command needs the program's toolchain" "$WORK/up.log"; then
  echo "!! the engine did not run command from the program image"
  exit 1
fi

echo "==> asserting how the engine attached command [$MODE_LABEL]"
ATTACH_LINE="$(grep 'oci: provider command running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
if [ -n "$ADDRESS_MODE" ]; then
  # The released command binary has no bind contract, so a successful attach at the
  # well-known port is itself the proof that the injected shim exec'd and served.
  if ! echo "$ATTACH_LINE" | grep -qE 'attaching at [^ ]*provider-command[^ ]*:7777'; then
    echo "!! expected the engine to attach by container DNS name at the well-known port :7777"
    exit 1
  fi
  if echo "$ATTACH_LINE" | grep -q '127.0.0.1'; then
    echo "!! the engine attached over loopback — address mode did not take effect"
    exit 1
  fi
else
  if ! echo "$ATTACH_LINE" | grep -q 'attaching at 127.0.0.1:'; then
    echo "!! expected the netns default: engine attaching over the shared loopback"
    exit 1
  fi
fi
# The discriminating control: jq is on the program image's PATH and in no provider
# image, and the shared mount carries files, not a toolchain. Empty/absent here means
# the provider ran from its own image (the command would have failed "jq: not found").
if [ "$TOOLCHAIN" != "$EXPECTED_TOOLCHAIN" ]; then
  echo "!! toolchain mismatch: got '${TOOLCHAIN:-<empty>}', want '$EXPECTED_TOOLCHAIN'"
  echo "   (the command provider did not get the program image's PATH — jq not found?)"
  exit 1
fi
echo "    toolchain = $TOOLCHAIN (jq ran from the program image's PATH)"

echo "==> asserting the command provider read the program's workspace"
if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  echo "   (the command provider did not read the program image's baked workspace)"
  exit 1
fi
echo "    marker = $MARKER (via the shared mount — any provider would see this)"

CRED="$("$WRAPPER" stack output cred)"
echo "==> asserting the engine's environment was projected onto the provider container"
if [ "$CRED" != "$EXPECTED_CRED" ]; then
  echo "!! cred mismatch: got '${CRED:-<empty>}', want '$EXPECTED_CRED'"
  echo "   (the engine's environment was not projected onto the command provider container)"
  exit 1
fi
echo "    cred = $CRED (projected from the engine env onto the provider container)"

RUNTIME_OUTPUT="$("$WRAPPER" stack output runtimeOutput)"
echo "==> asserting live volume sharing (program wrote a file at runtime, provider reads it)"
if [ "$RUNTIME_OUTPUT" != "written-at-runtime" ]; then
  echo "!! runtimeOutput mismatch: got '${RUNTIME_OUTPUT:-<empty>}', want 'written-at-runtime'"
  echo "   (the provider did not read a file the program wrote at runtime — volume sharing broken)"
  exit 1
fi
echo "    runtimeOutput = $RUNTIME_OUTPUT (live volume sharing confirmed)"
echo "==> run-from-program-image provider smoke test PASS [$MODE_LABEL] — toolchain + workspace + projected-env + live sharing"
