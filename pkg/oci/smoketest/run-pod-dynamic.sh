#!/usr/bin/env bash
#
# Dynamic-provider smoke test. Proves dynamic-provider execution in the OCI pod
# model across the full create -> destroy lifecycle: a pulumi.dynamic.Resource
# whose CRUD code is serialized from the program runs in a provider container
# started FROM THE PROGRAM IMAGE (the SDK's dynamic-provider entrypoint is native
# to it), with nothing injected — no provider image, no binary copy, no ensure
# step. The program image's bootstrap shim boots the dynamic-provider entrypoint
# when the engine sets PULUMI_OCI_ROLE.
#
# Discriminating proof (vs. a no-op test that would pass from any image): the
# dynamic provider's `create` reads /program-marker, a file baked into the PROGRAM
# image and present in no other, and returns it as the resource's output. If the
# provider had run from any other image the read would throw and `up` would fail.
# So a single assertion — stack output == the baked marker — proves both "the
# dynamic resource was created" and "its provider ran welded to the program
# image". We also assert the engine logged that it ran the dynamic provider from
# the program image.
#
# Then `destroy` exercises the harder, no-program-running path: at destroy the
# program never runs, so the engine must start the dynamic provider from the
# program image and deserialize the closure FROM STATE to call delete. The `delete`
# closure reads /program-marker too, so a successful destroy proves the provider
# again ran welded to the program image with no program in the picture.
#
# ADDRESS MODE (OCI_ADDRESS_MODE=1) runs the same proof over the address model,
# and it is the case the FORWARDER SHIM CANNOT SERVE: the dynamic provider is the
# program image's SDK entrypoint, which no registry proxy synthesizes a shim
# around. Reachability comes from the SDK bind contract alone — the engine sets
# PULUMI_PLUGIN_LISTEN_ADDRESS, the SDK binds 0.0.0.0:7777 in its own container
# on the pod network, and the engine attaches by container DNS name. The image
# carries NO shim binary (asserted), so a green run proves the engine wiring and
# the SDK's half of the contract end to end with no forwarding in the picture.
# Default (netns) mode doubles as the negative control: the variable is never set,
# the provider binds ephemeral loopback, and the engine attaches over 127.0.0.1
# exactly as before.
#
# Pipeline (mirrors run-pod-command.sh, with a Node program image):
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image and the Node program image (bakes /program-marker; carries THIS
#      BRANCH's Node SDK, whose dynamic-provider entrypoint honors the bind
#      contract — see the Dockerfile for why it overwrites the registry install)
#   2. drive `pulumi up` through the pulumi-pod wrapper, with the program image
#      forwarded as PULUMI_POD_PROGRAM_IMAGE
#   3. assert the dynamic provider ran from the program image and its output is the
#      baked marker
#
# Usage: run-pod-dynamic.sh                    # netns mode (provider shares the engine netns)
#        OCI_ADDRESS_MODE=1 run-pod-dynamic.sh # address mode (own container, attach by DNS, no shim)
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-node-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-node-dynamic"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live
REPO_ROOT="$SMOKE_DIR/../../.."
NODE_SDK_DIR="$REPO_ROOT/sdk/nodejs"

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node-dynamic:latest"
STACK="dev"
EXPECTED_MARKER="oci-dynamic-welded-to-program-image"

ADDRESS_MODE="${OCI_ADDRESS_MODE:-}"
MODE_LABEL="netns (provider shares the engine netns, dialed over 127.0.0.1)"
if [ -n "$ADDRESS_MODE" ]; then
  MODE_LABEL="address (provider in its own container, attached by DNS at :7777 via the SDK bind contract — no shim)"
fi

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the watcher, the staged SDK, and the scratch dir.
  if [ -n "${WATCH_PID:-}" ]; then kill "$WATCH_PID" >/dev/null 2>&1 || true; fi
  rm -rf "$PROGRAM_DIR/sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run dynamic-provider test"
  exit 1
fi

build_engine_image

# ── stage this branch's Node SDK into the build context ─────────────────────
# Rebuilt every run, not reused: a stale bin/ would ship an SDK without the change and the
# failure would look like the change being wrong rather than the artifact being old.
echo "==> building this branch's Node SDK (sdk/nodejs -> bin/)"
( cd "$NODE_SDK_DIR" && mise exec -- make build_package >/dev/null )
if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$NODE_SDK_DIR/bin/cmd/dynamic-provider/index.js"; then
  echo "!! the built Node SDK's dynamic-provider entrypoint does not honor the bind contract —"
  echo "   bin/ is stale or the build failed"
  exit 1
fi
if [ ! -f "$NODE_SDK_DIR/bin/cmd/run/index.js" ]; then
  echo "!! the built Node SDK is missing cmd/run, which oci-bootstrap.sh execs"; exit 1
fi
rm -rf "$PROGRAM_DIR/sdk-bin"
cp -R "$NODE_SDK_DIR/bin" "$PROGRAM_DIR/sdk-bin"
echo "   staged $(du -sh "$PROGRAM_DIR/sdk-bin" | cut -f1) of SDK into the build context"

echo "==> building Node program image $PROGRAM_IMAGE (bakes /program-marker, ships the dynamic-provider shim)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR"

if [ -n "$ADDRESS_MODE" ]; then
  # The whole point of the address-mode run: the program image must contain no forwarder
  # shim, so reachability can only come from the SDK binding the requested address itself.
  if docker run --rm --entrypoint sh "$PROGRAM_IMAGE" -c 'test -e /plugin/shim'; then
    echo "!! the program image contains /plugin/shim — this run must prove the SDK bind"
    echo "   contract with NO shim in the picture; remove the shim from the image"
    exit 1
  fi
  echo "    program image carries no /plugin/shim — reachability must come from the SDK bind contract"
  export PULUMI_POD_ADDRESS_MODE=1 # forwarded host->engine by the wrapper's env projection
fi

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Topology evidence: capture the dynamic-provider container's netns mode as it appears.
# The wrapper names containers pulumi-pod-<podid>-<logical>-<seq>, so filter on the
# logical name. Evidence only — the load-bearing assertions are on the engine's attach
# line and the marker output. Strays from a crashed earlier run would be captured
# instead of this run's container, so clear them first.
STRAYS="$(docker ps -aq --filter name=provider-pulumi-nodejs 2>/dev/null || true)"
if [ -n "$STRAYS" ]; then
  echo "    (removing stray provider containers from an earlier run)"
  docker rm -f $STRAYS >/dev/null 2>&1 || true
fi
( for _ in $(seq 1 600); do
    cname="$(docker ps -a --filter name=provider-pulumi-nodejs --format '{{.Names}}' 2>/dev/null | head -1)"
    if [ -n "$cname" ]; then
      docker inspect -f '{{.HostConfig.NetworkMode}}' "$cname" >"$WORK/provider-netmode" 2>/dev/null || true
      break
    fi
    sleep 0.1
  done ) &
WATCH_PID=$!

# Drive the deployment with the wrapper — it bootstraps the pod (network, engine
# container, PULUMI_POD_* contract, mounts, teardown) and defaults the backend +
# stack state into the mounted dir. PULUMI_POD_PROGRAM_IMAGE is forwarded so the
# engine's container host runs the dynamic provider from the program image.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"

echo "==> pulumi-pod [$MODE_LABEL]: stack init + up + output (engine runs the dynamic provider FROM the program image)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"
MARKER="$("$WRAPPER" stack output marker)"

echo "==> asserting the dynamic provider ran from the program image and produced the baked marker"
if ! grep -q 'oci: provider pulumi-nodejs is a dynamic provider' "$WORK/up.log"; then
  echo "!! the engine did not run the dynamic provider from the program image"
  exit 1
fi
if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  echo "   (the dynamic provider did not run welded to the program image)"
  exit 1
fi
echo "    marker = $MARKER"

echo "==> asserting how the engine attached [$MODE_LABEL]"
ATTACH_LINE="$(grep 'oci: provider pulumi-nodejs running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
NETMODE="$(cat "$WORK/provider-netmode" 2>/dev/null || true)"
if [ -n "$ADDRESS_MODE" ]; then
  if ! echo "$ATTACH_LINE" | grep -qE 'attaching at [^ ]*provider-pulumi-nodejs[^ ]*:7777'; then
    echo "!! expected the engine to attach by container DNS name at the well-known port :7777"
    exit 1
  fi
  if echo "$ATTACH_LINE" | grep -q '127.0.0.1'; then
    echo "!! the engine attached over loopback — address mode did not take effect"
    exit 1
  fi
  if [ -z "$NETMODE" ]; then
    echo "    (provider container was not caught in time — no NetworkMode recorded)"
  elif [ "${NETMODE#container:}" != "$NETMODE" ]; then
    echo "!! provider NetworkMode = $NETMODE — the provider shares another container's netns,"
    echo "   so this run proved nothing about reachability across namespaces"
    exit 1
  else
    echo "    provider NetworkMode = $NETMODE -> own netns on the pod network; the engine's"
    echo "    dial-back at <dns>:7777 crossed namespaces, served by the SDK's own bind"
  fi
else
  if ! echo "$ATTACH_LINE" | grep -q 'attaching at 127.0.0.1:'; then
    echo "!! expected the netns default: engine attaching over the shared loopback"
    exit 1
  fi
  if [ -n "$NETMODE" ] && [ "${NETMODE#container:}" = "$NETMODE" ]; then
    echo "!! provider NetworkMode = $NETMODE — expected it to share the engine's netns (container:...)"
    exit 1
  fi
  echo "    netns default intact: provider shares the engine netns (${NETMODE:-uncaught}), dialed over loopback"
fi

echo "==> pulumi-pod: destroy (NO program runs — the dynamic provider must start from the"
echo "    program image and delete the resource from state)"
"$WRAPPER" destroy --yes --skip-preview 2>&1 | tee "$WORK/destroy.log"

echo "==> asserting destroy started the dynamic provider from the program image (no-program path)"
if ! grep -q 'oci: provider pulumi-nodejs is a dynamic provider' "$WORK/destroy.log"; then
  echo "!! destroy did not start the dynamic provider from the program image"
  exit 1
fi
if [ -n "$ADDRESS_MODE" ] &&
  ! grep -qE 'attaching at [^ ]*provider-pulumi-nodejs[^ ]*:7777' "$WORK/destroy.log"; then
  echo "!! destroy did not attach by container DNS at :7777 — the no-program path fell off address mode"
  exit 1
fi
# The delete closure reads /program-marker; because the destroy above succeeded
# (set -o pipefail aborts the script otherwise), the provider deserialized the
# closure from state and ran it welded to the program image — with no program
# running. Confirm the resource was actually deleted, not skipped.
if ! grep -q 'deleted' "$WORK/destroy.log"; then
  echo "!! destroy did not report deleting the dynamic resource"
  exit 1
fi
echo "==> dynamic-provider smoke test PASS [$MODE_LABEL] — dynamic providers run from the program image at create AND destroy"
