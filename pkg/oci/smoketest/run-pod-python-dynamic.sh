#!/usr/bin/env bash
#
# Python dynamic-provider smoke test — the language-agnostic counterpart to
# run-pod-dynamic.sh (Node). Proves the container host's dynamic-provider third
# branch is not Node-specific: a Python pulumi.dynamic.Resource whose CRUD code is
# serialized from the program (via dill) runs in a provider container started FROM
# THE PROGRAM IMAGE, its bootstrap shim booting `python -m pulumi.dynamic` when the
# engine sets PULUMI_OCI_ROLE=dynamic-provider. Nothing is injected.
#
# Discriminating proof: the provider's `create` reads /program-marker, a file baked
# into the PROGRAM image and present in no other, and returns it as the resource's
# output. From any other image the read would throw and `up` would fail. So stack
# output == the baked marker proves both "the dynamic resource was created" and "its
# provider ran welded to the program image".
#
# ADDRESS MODE (OCI_ADDRESS_MODE=1) runs the same proof over the address model, and
# like the Node test it is a case the FORWARDER SHIM CANNOT SERVE: the dynamic
# provider is the program image's SDK entrypoint, which no registry proxy
# synthesizes a shim around. Reachability comes from the SDK bind contract alone —
# the engine sets PULUMI_PLUGIN_LISTEN_ADDRESS, the PYTHON SDK's dynamic entrypoint
# (pulumi/dynamic/__main__.py) binds 0.0.0.0:7777 in its own container on the pod
# network, and the engine attaches by container DNS name. The image carries NO shim
# binary (asserted), so a green run proves the Python SDK's half of the contract
# with no forwarding in the picture. The program image must therefore carry THIS
# BRANCH's Python SDK — staged install-then-overwrite, exactly as the transform
# test does (see program-python-transform/Dockerfile for the reasoning).
# Default (netns) mode doubles as the negative control: the variable is never set,
# the provider binds ephemeral loopback, and the engine attaches over 127.0.0.1.
#
# The Python program image needs the SDK's program-exec shim
# (pulumi-language-python-exec) to launch the program; that ships with the CLI, not
# the pip package, so we copy it from the repo into the build context.
#
# Usage: run-pod-python-dynamic.sh                    # netns mode (provider shares the engine netns)
#        OCI_ADDRESS_MODE=1 run-pod-python-dynamic.sh # address mode (own container, attach by DNS, no shim)
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-python-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-python-dynamic"
PKG_DIR="$SMOKE_DIR/../.."           # the pkg/ Go module, where the CLI + host live
REPO_ROOT="$SMOKE_DIR/../../.."      # repo root, for the python exec shim + SDK under sdk/
EXEC_SHIM="$REPO_ROOT/sdk/python/cmd/pulumi-language-python-exec"
PY_SDK_LIB="$REPO_ROOT/sdk/python/lib/pulumi"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-python-dynamic:latest"
STACK="dev"
EXPECTED_MARKER="oci-dynamic-welded-to-python-image"

ADDRESS_MODE="${OCI_ADDRESS_MODE:-}"
MODE_LABEL="netns (provider shares the engine netns, dialed over 127.0.0.1)"
if [ -n "$ADDRESS_MODE" ]; then
  MODE_LABEL="address (provider in its own container, attached by DNS at :7777 via the SDK bind contract — no shim)"
fi

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/ctx" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod; this only clears the watcher, the cross-compiled
  # binaries, and the scratch dir (the staged SDK lives inside it).
  if [ -n "${WATCH_PID:-}" ]; then kill "$WATCH_PID" >/dev/null 2>&1 || true; fi
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run Python dynamic-provider test"
  exit 1
fi

build_engine_image

# ── stage this branch's Python SDK into the build context ───────────────────
# The image overlays it onto the pip install (install-then-overwrite; see the
# Dockerfile). Guarded on the dynamic entrypoint's bind-contract line: without the
# branch SDK, address mode fails the engine's handshake honesty check (the stock
# PyPI SDK ignores PULUMI_PLUGIN_LISTEN_ADDRESS), and the failure would look like
# the engine being wrong rather than the artifact being stock.
echo "==> staging this branch's Python SDK (sdk/python/lib/pulumi)"
if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$PY_SDK_LIB/dynamic/__main__.py"; then
  echo "!! the branch Python SDK's dynamic entrypoint does not honor the bind contract —"
  echo "   $PY_SDK_LIB/dynamic/__main__.py is not the version under test"
  exit 1
fi
cp -R "$PY_SDK_LIB" "$WORK/ctx/sdk-lib"
find "$WORK/ctx/sdk-lib" -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null || true
echo "   staged $(du -sh "$WORK/ctx/sdk-lib" | cut -f1) of SDK into the build context"

echo "==> assembling Python program build context (program + the SDK program-exec shim)"
cp "$PROGRAM_DIR"/* "$WORK/ctx/"
cp "$EXEC_SHIM" "$WORK/ctx/"

echo "==> building Python program image $PROGRAM_IMAGE (pip install pulumi + branch SDK overlay, bakes /program-marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$WORK/ctx"

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
else
  # The wrapper defaults address mode ON; the netns run must pin the legacy mode
  # explicitly (empty = netns, per the wrapper contract) or it silently tests the
  # wrong topology.
  export PULUMI_POD_ADDRESS_MODE=
fi

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Topology evidence: capture the dynamic-provider container's netns mode as it appears.
# The wrapper names containers pulumi-pod-<podid>-<logical>-<seq>, so filter on the
# logical name. Evidence only — the load-bearing assertions are on the engine's attach
# line and the marker output. Strays from a crashed earlier run would be captured
# instead of this run's container, so clear them first.
STRAYS="$(docker ps -aq --filter name=provider-pulumi-python 2>/dev/null || true)"
if [ -n "$STRAYS" ]; then
  echo "    (removing stray provider containers from an earlier run)"
  docker rm -f $STRAYS >/dev/null 2>&1 || true
fi
( for _ in $(seq 1 600); do
    cname="$(docker ps -a --filter name=provider-pulumi-python --format '{{.Names}}' 2>/dev/null | head -1)"
    if [ -n "$cname" ]; then
      docker inspect -f '{{.HostConfig.NetworkMode}}' "$cname" >"$WORK/provider-netmode" 2>/dev/null || true
      break
    fi
    sleep 0.1
  done ) &
WATCH_PID=$!

export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"

echo "==> pulumi-pod [$MODE_LABEL]: stack init + up + output (engine runs the Python dynamic provider FROM the program image)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"
MARKER="$("$WRAPPER" stack output marker)"

echo "==> asserting the Python dynamic provider ran from the program image and produced the baked marker"
if ! grep -q 'oci: provider pulumi-python is a dynamic provider' "$WORK/up.log"; then
  echo "!! the engine did not run the Python dynamic provider from the program image"
  exit 1
fi
if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  echo "   (the Python dynamic provider did not run welded to the program image)"
  exit 1
fi
echo "    marker = $MARKER"

echo "==> asserting how the engine attached [$MODE_LABEL]"
ATTACH_LINE="$(grep 'oci: provider pulumi-python running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
NETMODE="$(cat "$WORK/provider-netmode" 2>/dev/null || true)"
if [ -n "$ADDRESS_MODE" ]; then
  if ! echo "$ATTACH_LINE" | grep -qE 'attaching at [^ ]*provider-pulumi-python[^ ]*:7777'; then
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
    echo "    dial at <dns>:7777 crossed namespaces, served by the Python SDK's own bind"
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
echo "==> Python dynamic-provider smoke test PASS [$MODE_LABEL] — dynamic-provider execution is language-agnostic"
