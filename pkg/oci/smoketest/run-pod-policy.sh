#!/usr/bin/env bash
#
# Policy (analyzer) smoke test. Proves that a policy pack runs as a container in the
# OCI pod model: the engine resolves a pack declaring `runtime: oci` to its image,
# starts it as a pod member, and drives its Analyzer gRPC surface
# (GetAnalyzerInfo/Analyze) — over the shared loopback in the netns default, or by
# container DNS at the well-known port in address mode — the analyzer analogue
# of how MLCs run as containers. The one new bit vs. providers is the analyzer
# protocol: the pack is a server the engine calls (no Attach RPC), so the host dials
# it and hands the engine a client (plugin.NewAnalyzerWithClient), raising the gRPC
# message-size limit to match the engine's other plugin connections.
#
# This directly exercises the failure mode the OCI model fixes for policy: an org
# can ship a policy pack in any language, but in practice it breaks when the consumer
# lacks the author's toolchain (e.g. Pulumi's own internal infrastructure-policy pack
# silently needs a particular Node/ts-node, undocumented). Here the pack is
# TypeScript, compiled by ts-node at run time. ts-node + Node live in the POLICY
# image; the engine image (alpine, no Node) has neither — so the pack runs only
# because its toolchain is baked into its own container.
#
# Discriminating proof (vs. a no-op that would pass from any image): the pack's
# validateResource reads /policy-marker — a file baked into the POLICY image alone —
# inside its validation logic, and reports it in the violation message. Had the
# policy run ambiently (in the engine image) the read would throw. So the marker
# appearing in the violation proves the policy logic ran from its own image. We also
# assert the engine logged that it started the pack as a container.
#
# Refs are the currency, not paths. The host resolves the pack to its image ref
# BEFORE the engine sees it (here: read straight off the pack's PulumiPolicy.yaml on
# the host, where the dir lives natively — no mount), and passes the *ref* as
# --policy-pack. The engine consumes the ref exactly like a provider's image and
# reads no manifest off a mount. So nothing projects PulumiPolicy.yaml into the
# engine — we assert that too: a path is a dev-time input, a ref is what crosses the
# boundary. (A local dir still works for dev convenience; it is just not the form the
# engine depends on.)
#
# The companion program is the dynamic-resource program (reused): it registers a
# pulumi-nodejs:dynamic:Resource, which the pack flags. Enforcement is advisory, so
# `up` succeeds and prints the violation.
#
# Pipeline (mirrors run-pod-dynamic.sh, plus a policy image):
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image, the Node program image, and the TypeScript policy image
#   2. drive `pulumi up --policy-pack` through the pulumi-pod wrapper
#   3. assert the pack ran as a container and its violation carries the baked marker
#
# ADDRESS MODE (OCI_ADDRESS_MODE=1) runs the same proof over the address model: the
# pack runs in its OWN container on the pod network (not the engine's netns), the
# engine asks it to bind the well-known port via PULUMI_PLUGIN_LISTEN_ADDRESS, and
# attaches by container DNS at :7777. The policy SDK's serve site honors that env
# only with the bind-contract patch (pulumi/pulumi-policy), so this run stages the
# patched @pulumi/policy from a local clone over the stock install — reachability
# can only come from the SDK binding the requested address itself; no shim exists
# on the policy path at all.
#
# Usage: run-pod-policy.sh                    # netns default (shared loopback)
#        OCI_ADDRESS_MODE=1 run-pod-policy.sh # address mode (own container, DNS:7777)
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile);
# address mode also needs a pulumi/pulumi-policy clone (OCI_POLICY_SDK_DIR overrides
# the default ~/src/pulumi/pulumi-policy).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-node-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-node-dynamic"
POLICY_DIR="$SMOKE_DIR/policy-pack-node"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live
NODE_SDK_DIR="$SMOKE_DIR/../../../sdk/nodejs"

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node-dynamic:latest"
POLICY_IMAGE="oci-smoke-policy:latest"
STACK="dev"
EXPECTED_MARKER="oci-policy-ran-from-its-own-image"

ADDRESS_MODE="${OCI_ADDRESS_MODE:-}"
MODE_LABEL="netns (pack shares the engine netns, dialed over 127.0.0.1)"
if [ -n "$ADDRESS_MODE" ]; then
  MODE_LABEL="address (pack in its own container, attached by DNS at :7777 via the SDK bind contract)"
fi

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the watcher, the staged policy SDK, and the scratch dir.
  if [ -n "${WATCH_PID:-}" ]; then kill "$WATCH_PID" >/dev/null 2>&1 || true; fi
  rm -rf "$POLICY_DIR/policy-sdk-bin"
  rm -rf "$PROGRAM_DIR/sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run policy test"
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

# ── stage the bind-contract @pulumi/policy into the policy build context ──────
# The policy SDK lives in its own repo (pulumi/pulumi-policy); the bind-contract
# patch — the serve site honoring PULUMI_PLUGIN_LISTEN_ADDRESS — is on a local
# clone. Install-then-overwrite: the image npm-installs the stock SDK, then
# overlays the patched compiled output on top, so there is never a second nested
# copy for the pack to resolve instead. Rebuilt every run, not reused: a stale
# bin/ would ship an SDK without the change and the failure would look like the
# change being wrong rather than the artifact being old. Without a clone the
# netns run proceeds on the stock SDK (the image is byte-identical to before);
# address mode cannot work stock, so it fails fast here with the reason.
POLICY_SDK_DIR="${OCI_POLICY_SDK_DIR:-$HOME/src/pulumi/pulumi-policy}/sdk/nodejs/policy"
rm -rf "$POLICY_DIR/policy-sdk-bin"
mkdir -p "$POLICY_DIR/policy-sdk-bin"
if [ -d "$POLICY_SDK_DIR" ]; then
  echo "==> building the bind-contract @pulumi/policy ($POLICY_SDK_DIR -> bin/)"
  (cd "$POLICY_SDK_DIR" && bun install >/dev/null && bun run tsc >/dev/null)
  if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$POLICY_SDK_DIR/bin/server.js"; then
    echo "!! the built policy SDK's serve site does not honor the bind contract —"
    echo "   bin/ is stale or the clone lacks the patch"
    exit 1
  fi
  cp -R "$POLICY_SDK_DIR/bin/." "$POLICY_DIR/policy-sdk-bin/"
  # Keep the stock package.json: the overlay is code, not identity (the built one
  # carries an unsubstituted ${VERSION} placeholder besides).
  rm -f "$POLICY_DIR/policy-sdk-bin/package.json"
  echo "   staged $(du -sh "$POLICY_DIR/policy-sdk-bin" | cut -f1) of policy SDK into the build context"
elif [ -n "$ADDRESS_MODE" ]; then
  echo "!! address mode needs the bind-contract @pulumi/policy and no clone was found at $POLICY_SDK_DIR"
  echo "   (git get pulumi/pulumi-policy, or point OCI_POLICY_SDK_DIR at a clone)"
  exit 1
else
  echo "==> no pulumi-policy clone at $POLICY_SDK_DIR — netns run uses the stock @pulumi/policy"
fi

echo "==> building TypeScript policy image $POLICY_IMAGE (ts-node toolchain + /policy-marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$POLICY_IMAGE" -f "$POLICY_DIR/Dockerfile" "$POLICY_DIR"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Resolve the pack to its image ref HOST-SIDE, off the pack's PulumiPolicy.yaml where
# the dir lives natively (no mount, no engine involvement). This is the path->ref
# boundary: the engine will receive only the ref. Nothing about the pack's manifest
# is projected into the engine mount.
POLICY_REF="$(sed -n 's/^[[:space:]]*image:[[:space:]]*//p' "$POLICY_DIR/PulumiPolicy.yaml")"
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
STRAYS="$(docker ps -aq --filter name=policy-oci-smoke-policy 2>/dev/null || true)"
if [ -n "$STRAYS" ]; then
  echo "    (removing stray policy containers from an earlier run)"
  docker rm -f $STRAYS >/dev/null 2>&1 || true
fi
( for _ in $(seq 1 600); do
    cname="$(docker ps -a --filter name=policy-oci-smoke-policy --format '{{.Names}}' 2>/dev/null | head -1)"
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
if [ -n "$ADDRESS_MODE" ]; then
  export PULUMI_POD_ADDRESS_MODE=1 # forwarded host->engine by the wrapper's env projection
else
  # The wrapper defaults address mode ON; the netns run must pin the legacy mode
  # explicitly (empty = netns, per the wrapper contract) or it silently tests the
  # wrong topology.
  export PULUMI_POD_ADDRESS_MODE=
fi

echo "==> pulumi-pod [$MODE_LABEL]: stack init + up --policy-pack <ref> (engine consumes the ref, not a path)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview --policy-pack "$POLICY_REF" 2>&1 | tee "$WORK/up.log"

echo "==> asserting no PulumiPolicy.yaml was projected into the engine mount"
if find "$WORK/project" -name PulumiPolicy.yaml | grep -q .; then
  echo "!! a PulumiPolicy.yaml reached the engine mount — the ref form should need none"
  exit 1
fi
echo "    confirmed: the engine ran the pack from a ref alone, no manifest projected"

echo "==> asserting the engine ran the policy pack as a container"
if ! grep -q 'oci: policy pack' "$WORK/up.log"; then
  echo "!! the engine did not start the policy pack as a container"
  exit 1
fi

echo "==> asserting how the engine attached the pack [$MODE_LABEL]"
ATTACH_LINE="$(grep 'oci: policy pack .* running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
NETMODE="$(cat "$WORK/policy-netmode" 2>/dev/null || true)"
if [ -n "$ADDRESS_MODE" ]; then
  if ! echo "$ATTACH_LINE" | grep -qE 'attaching at [^ ]*policy-oci-smoke-policy[^ ]*:7777'; then
    echo "!! expected the engine to attach by container DNS name at the well-known port :7777"
    exit 1
  fi
  if echo "$ATTACH_LINE" | grep -q '127.0.0.1'; then
    echo "!! the engine attached over loopback — address mode did not take effect"
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
    echo "    Analyze calls at <dns>:7777 crossed namespaces, served by the SDK's own bind"
  fi
else
  if ! echo "$ATTACH_LINE" | grep -q 'attaching at 127.0.0.1:'; then
    echo "!! expected the netns default: engine attaching over the shared loopback"
    exit 1
  fi
  if [ -n "$NETMODE" ] && [ "${NETMODE#container:}" = "$NETMODE" ]; then
    echo "!! policy NetworkMode = $NETMODE — expected it to share the engine's netns (container:...)"
    exit 1
  fi
  echo "    netns default intact: pack shares the engine netns (${NETMODE:-uncaught}), dialed over loopback"
fi

echo "==> asserting the policy ran from its own image (violation carries the baked marker)"
if ! grep -q "marker=$EXPECTED_MARKER" "$WORK/up.log"; then
  echo "!! expected policy violation with marker=$EXPECTED_MARKER not found"
  echo "   (the policy did not run from its image, or never evaluated the dynamic resource)"
  exit 1
fi
echo "    found violation with marker=$EXPECTED_MARKER"
echo "==> policy smoke test PASS [$MODE_LABEL] — a policy pack runs as a container, with its own toolchain, and analyzes resources"
