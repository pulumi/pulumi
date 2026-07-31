#!/usr/bin/env bash
#
# Python policy (analyzer) smoke test — the Python twin of run-pod-policy.sh. The
# engine's policy path is language-agnostic (an image run with
# PULUMI_OCI_ROLE=policy-pack, attached at the container's address on :7777), and
# the Node test already proves that path. What THIS test discriminates is the
# PYTHON policy SDK's serve site (pulumi_policy/policy.py in
# pulumi/pulumi-policy): the pack is reachable only if that serve site honors
# PULUMI_PLUGIN_LISTEN_ADDRESS itself — no shim exists on the policy path, and a
# Python pack is ITS OWN server (PolicyPack() binds and serves from __init__;
# there is no run-policy-pack harness between the bootstrap and the SDK).
#
# The companion program is the NODE dynamic-resource program — deliberately the
# same companion the Node policy test uses, staging and all, so the Python policy
# image is the single new variable in the run.
#
# Discriminating proof, as in the Node test: the pack's validate reads
# /policy-marker — baked into the PYTHON POLICY image alone — inside its
# validation logic and reports it in the violation message. The engine image
# carries no Python at all, so the pack also cannot have run ambiently.
#
# Usage: run-pod-policy-python.sh
# Requires a running Docker daemon, the repo Go toolchain (to cross-compile), and
# a pulumi/pulumi-policy clone (OCI_POLICY_SDK_DIR overrides the default
# ~/src/pulumi/pulumi-policy).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-node-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-node-dynamic"
POLICY_DIR="$SMOKE_DIR/policy-pack-python"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live
NODE_SDK_DIR="$SMOKE_DIR/../../../sdk/nodejs"

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node-dynamic:latest"
POLICY_IMAGE="oci-smoke-policy-py:latest" # short: the sanitized ref feeds a dialable DNS label
STACK="dev"
EXPECTED_MARKER="oci-python-policy-ran-from-its-own-image"

MODE_LABEL="pack in its own container, attached at IP:7777 via the SDK bind contract"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the watcher, the staged policy SDK, and the scratch dir.
  if [ -n "${WATCH_PID:-}" ]; then kill "$WATCH_PID" >/dev/null 2>&1 || true; fi
  rm -rf "$POLICY_DIR/policy-sdk-lib"
  rm -rf "$PROGRAM_DIR/sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run Python policy test"
  exit 1
fi

build_engine_image

# Stage this branch's Node SDK into the program build context — the companion
# program image's Dockerfile overlays it onto the stock install (the same
# install-then-overwrite the dynamic test uses; its build fails without it).
# Rebuilt every run so a stale bin/ can't masquerade as a change being wrong.
echo "==> building this branch's Node SDK (sdk/nodejs -> bin/)"
(cd "$NODE_SDK_DIR" && mise exec -- make build_package >/dev/null)
if [ ! -f "$NODE_SDK_DIR/bin/cmd/run/index.js" ]; then
  echo "!! the built Node SDK is missing cmd/run, which oci-bootstrap.sh execs"; exit 1
fi
rm -rf "$PROGRAM_DIR/sdk-bin"
cp -R "$NODE_SDK_DIR/bin" "$PROGRAM_DIR/sdk-bin"

echo "==> building Node program image $PROGRAM_IMAGE (registers a dynamic resource)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR"

# ── stage the bind-contract pulumi_policy into the policy build context ───────
# The policy SDK lives in its own repo (pulumi/pulumi-policy); the bind-contract
# patch — the serve site honoring PULUMI_PLUGIN_LISTEN_ADDRESS — is on a local
# clone. Install-then-overwrite: the image pip-installs the stock SDK, then
# overlays the patched source on top, so there is never a second nested copy for
# the pack to resolve instead. Python needs no build step (lib/pulumi_policy IS
# the shipped package), so the grep-guard checks the clone source directly.
# Without a clone the netns run proceeds on the stock SDK (the image is
# byte-identical to before); address mode cannot work stock, so it fails fast
# here with the reason.
POLICY_SDK_LIB="${OCI_POLICY_SDK_DIR:-$HOME/src/pulumi/pulumi-policy}/sdk/python/lib/pulumi_policy"
rm -rf "$POLICY_DIR/policy-sdk-lib"
mkdir -p "$POLICY_DIR/policy-sdk-lib"
if [ -d "$POLICY_SDK_LIB" ]; then
  if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$POLICY_SDK_LIB/policy.py"; then
    echo "!! the clone's pulumi_policy serve site does not honor the bind contract —"
    echo "   $POLICY_SDK_LIB/policy.py lacks the patch"
    exit 1
  fi
  echo "==> staging the bind-contract pulumi_policy ($POLICY_SDK_LIB)"
  cp -R "$POLICY_SDK_LIB/." "$POLICY_DIR/policy-sdk-lib/"
  # Keep the stock version.py: the overlay is code, not identity (the clone's is
  # an unsubstituted "1.0.0" placeholder besides).
  rm -f "$POLICY_DIR/policy-sdk-lib/version.py"
  find "$POLICY_DIR/policy-sdk-lib" -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null || true
  echo "   staged $(du -sh "$POLICY_DIR/policy-sdk-lib" | cut -f1) of policy SDK into the build context"
else
  echo "!! this test needs the bind-contract pulumi_policy and no clone was found at $POLICY_SDK_LIB"
  echo "   (git get pulumi/pulumi-policy, or point OCI_POLICY_SDK_DIR at a clone)"
  exit 1
fi

echo "==> building Python policy image $POLICY_IMAGE (python toolchain + /policy-marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$POLICY_IMAGE" -f "$POLICY_DIR/Dockerfile" "$POLICY_DIR"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Resolve the pack to its image ref HOST-SIDE, off the pack's PulumiPolicy.yaml where
# the dir lives natively (no mount, no engine involvement) — the ref is what crosses
# into the engine, exactly as in the Node test.
POLICY_REF="$(sed -n 's/^[[:space:]]*image:[[:space:]]*//p' "$POLICY_DIR/PulumiPolicy.yaml" | head -1)"
if [ -z "$POLICY_REF" ]; then
  echo "!! could not resolve the policy pack image ref from $POLICY_DIR/PulumiPolicy.yaml"
  exit 1
fi
echo "==> resolved policy pack -> image ref (host-side): $POLICY_REF"

# Topology evidence: capture the policy container's netns mode as it appears. The
# wrapper names containers pulumi-pod-<podid>-<logical>-<seq>; the pack's logical
# name is policy-<sanitized ref>. Evidence only — the load-bearing assertions are
# on the engine's attach line and the marker. Strays from a crashed earlier run
# would be captured instead of this run's container, so clear them first.
STRAYS="$(docker ps -aq --filter name=policy-oci-smoke-policy-py 2>/dev/null || true)"
if [ -n "$STRAYS" ]; then
  echo "    (removing stray policy containers from an earlier run)"
  docker rm -f $STRAYS >/dev/null 2>&1 || true
fi
( for _ in $(seq 1 600); do
    cname="$(docker ps -a --filter name=policy-oci-smoke-policy-py --format '{{.Names}}' 2>/dev/null | head -1)"
    if [ -n "$cname" ]; then
      docker inspect -f '{{.HostConfig.NetworkMode}}' "$cname" >"$WORK/policy-netmode" 2>/dev/null || true
      break
    fi
    sleep 0.1
  done ) &
WATCH_PID=$!

# Drive the deployment with the wrapper — it bootstraps the pod (network, engine
# container, PULUMI_POD_* contract, mounts, teardown) and defaults the backend +
# stack state into the mounted dir.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"
echo "==> pulumi-pod [$MODE_LABEL]: stack init + up --policy-pack <ref> (engine consumes the ref, not a path)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview --policy-pack "$POLICY_REF" 2>&1 | tee "$WORK/up.log"

echo "==> asserting the engine ran the policy pack as a container"
if ! grep -q 'oci: policy pack' "$WORK/up.log"; then
  echo "!! the engine did not start the policy pack as a container"
  exit 1
fi

echo "==> asserting how the engine attached the pack [$MODE_LABEL]"
ATTACH_LINE="$(grep 'oci: policy pack .* running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
NETMODE="$(cat "$WORK/policy-netmode" 2>/dev/null || true)"
if ! echo "$ATTACH_LINE" | grep -qE 'attaching at [0-9.]+:7777'; then
  echo "!! expected the engine to attach at the container's address on the well-known port :7777"
  exit 1
fi
if echo "$ATTACH_LINE" | grep -q '127.0.0.1'; then
  echo "!! the engine attached over loopback — the pack is not being dialed on the pod network"
  exit 1
fi
if [ -z "$NETMODE" ]; then
  echo "    (policy container was not caught in time — no NetworkMode recorded)"
elif [ "${NETMODE#container:}" != "$NETMODE" ]; then
  echo "!! policy NetworkMode = $NETMODE — the pack shares another container's netns,"
  echo "   so this run proved nothing about reachability across namespaces"
  exit 1
else
  echo "    policy NetworkMode = $NETMODE -> own netns on the pod network; the engine's"
  echo "    Analyze calls at IP:7777 crossed namespaces, served by the Python SDK's own bind"
fi

echo "==> asserting the policy ran from its own image (violation carries the baked marker)"
if ! grep -q "marker=$EXPECTED_MARKER" "$WORK/up.log"; then
  echo "!! expected policy violation with marker=$EXPECTED_MARKER not found"
  echo "   (the policy did not run from its image, or never evaluated the dynamic resource)"
  exit 1
fi
echo "    found violation with marker=$EXPECTED_MARKER"
echo "==> Python policy smoke test PASS [$MODE_LABEL] — a Python policy pack serves from its own image, its SDK binding the address the engine asks for"
