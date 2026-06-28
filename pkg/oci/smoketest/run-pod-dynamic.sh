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
# Pipeline (mirrors run-pod-command.sh, with a Node program image):
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image and the Node program image (bakes /program-marker, ships the shim)
#   2. drive `pulumi up` through the pulumi-pod wrapper, with the program image
#      forwarded as PULUMI_POD_PROGRAM_IMAGE
#   3. assert the dynamic provider ran from the program image and its output is the
#      baked marker
#
# Usage: run-pod-dynamic.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-node-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-node-dynamic"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node-dynamic:latest"
STACK="dev"
EXPECTED_MARKER="oci-dynamic-welded-to-program-image"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod (containers, volumes, network) itself; this only
  # clears the cross-compiled binaries and the scratch dir.
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run dynamic-provider test"
  exit 1
fi

build_engine_image

echo "==> building Node program image $PROGRAM_IMAGE (bakes /program-marker, ships the dynamic-provider shim)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Drive the deployment with the wrapper — it bootstraps the pod (network, engine
# container, PULUMI_POD_* contract, mounts, teardown) and defaults the backend +
# stack state into the mounted dir. PULUMI_POD_PROGRAM_IMAGE is forwarded so the
# engine's container host runs the dynamic provider from the program image.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"

echo "==> pulumi-pod: stack init + up + output (engine runs the dynamic provider FROM the program image)"
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

echo "==> pulumi-pod: destroy (NO program runs — the dynamic provider must start from the"
echo "    program image and delete the resource from state)"
"$WRAPPER" destroy --yes --skip-preview 2>&1 | tee "$WORK/destroy.log"

echo "==> asserting destroy started the dynamic provider from the program image (no-program path)"
if ! grep -q 'oci: provider pulumi-nodejs is a dynamic provider' "$WORK/destroy.log"; then
  echo "!! destroy did not start the dynamic provider from the program image"
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
echo "==> dynamic-provider smoke test PASS — dynamic providers run from the program image at create AND destroy"
