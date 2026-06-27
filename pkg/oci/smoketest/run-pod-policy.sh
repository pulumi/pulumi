#!/usr/bin/env bash
#
# Policy (analyzer) smoke test. Proves that a policy pack runs as a container in the
# OCI pod model: the engine resolves a pack declaring `runtime: oci` to its image,
# starts it as a pod member sharing the engine netns, and drives its Analyzer gRPC
# surface (GetAnalyzerInfo/Analyze) over the shared loopback — the analyzer analogue
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
# Usage: run-pod-policy.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-node-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-node-dynamic"
POLICY_DIR="$SMOKE_DIR/policy-pack-node"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

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

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project/policy"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the scratch dir.
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run policy test"
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

echo "==> building Node program image $PROGRAM_IMAGE (registers a dynamic resource)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR"

echo "==> building TypeScript policy image $POLICY_IMAGE (ts-node toolchain + /policy-marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$POLICY_IMAGE" -f "$POLICY_DIR/Dockerfile" "$POLICY_DIR"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
# Only the pack's manifest needs to be reachable in the engine container; its code
# lives in the policy image. Place it at /workspace/policy and point --policy-pack
# at it (relative to the engine container's workdir, /workspace).
cp "$POLICY_DIR/PulumiPolicy.yaml" "$WORK/project/policy/"

# Drive the deployment with the wrapper — it bootstraps the pod (network, engine
# container, PULUMI_POD_* contract, mounts, teardown) and defaults the backend +
# stack state into the mounted dir.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"

echo "==> pulumi-pod: stack init + up --policy-pack (engine runs the policy pack FROM its image)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview --policy-pack policy 2>&1 | tee "$WORK/up.log"

echo "==> asserting the engine ran the policy pack as a container"
if ! grep -q 'oci: policy pack' "$WORK/up.log"; then
  echo "!! the engine did not start the policy pack as a container"
  exit 1
fi

echo "==> asserting the policy ran from its own image (violation carries the baked marker)"
if ! grep -q "marker=$EXPECTED_MARKER" "$WORK/up.log"; then
  echo "!! expected policy violation with marker=$EXPECTED_MARKER not found"
  echo "   (the policy did not run from its image, or never evaluated the dynamic resource)"
  exit 1
fi
echo "    found violation with marker=$EXPECTED_MARKER"
echo "==> policy smoke test PASS — a policy pack runs as a container, with its own toolchain, and analyzes resources"
