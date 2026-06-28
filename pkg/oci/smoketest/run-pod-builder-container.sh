#!/usr/bin/env bash
#
# Builder-container smoke test: the OCI language host builds the program image in a
# *dedicated builder container*, not inside the engine container.
#
# This is the build/run seam (design: "Topology — the build phase"). Previously the
# build ran in-process inside the engine container — which only worked because the
# engine image happens to ship the docker CLI. Now `build: {image, command}` runs the
# command in a builder container whose image supplies the toolchain. The source +
# docker socket reach the builder via `--volumes-from` the engine container (same
# paths, no host-path translation); the socket is the artifact sink (load into the
# shared daemon).
#
# The test DISCRIMINATES: the builder image carries a marker (/only-in-builder) the
# engine image lacks, and the build command guards on it. So the build can only
# succeed if it ran in the builder container — a regression that routed it back
# in-process would fail loudly rather than pass green.
#
# Pipeline:
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine image
#   2. build the discriminating builder image (docker CLI + the marker)
#   3. assemble /project = Node program source + a structured-build Pulumi.yaml
#   4. run `pulumi up`; the language host builds the image in the builder container
#   5. assert the builder path ran (its log line), the program ran (stack output),
#      and the built image landed in the daemon
#
# Usage: run-pod-builder-container.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-builder-container"
PROGRAM_DIR="$SMOKE_DIR/program-node" # self-contained Node program (build needs no host step)
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
BUILDER_IMAGE="oci-smoke-builder:latest"            # the dedicated builder image
BUILT_IMAGE="oci-smoke-buildercontainer:latest"     # what the builder produces
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
EXPECTED_GREETING="hello-from-node-in-a-pod"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run builder-container test"
  exit 1
fi

# Start clean so the post-run image-inspect proves THIS run's builder built it.
docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true

build_engine_image

echo "==> building discriminating builder image $BUILDER_IMAGE (docker CLI + /only-in-builder marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$BUILDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.builder" "$SMOKE_DIR"

# Assemble the mounted project: the Node program's build context plus the
# structured-build Pulumi.yaml. NO program image is pre-built.
echo "==> assembling /project (program source + structured-build Pulumi.yaml)"
cp "$PROGRAM_DIR"/* "$WORK/project/"
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

echo "==> running engine container $ENGINE_NAME (language host builds the image in the builder container)"
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
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  -e STACK="$STACK" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select --create "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the build ran in the builder container, then the program ran"
# The container-path log line names the builder image; the legacy in-process path
# logs "oci: building program image:" without "in builder". Asserting on this proves
# the structured (builder-container) path was taken.
if ! grep -q "oci: building program image in builder $BUILDER_IMAGE" "$WORK/engine.log"; then
  echo "!! the language host did not run the build in the builder container"
  exit 1
fi
if ! grep -q "oci: built program image" "$WORK/engine.log"; then
  echo "!! the builder did not produce an image ref"
  exit 1
fi
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ "$GREETING" != "$EXPECTED_GREETING" ]; then
  echo "!! greeting mismatch: got '${GREETING:-<empty>}', want '$EXPECTED_GREETING'"
  exit 1
fi
echo "    greeting = $GREETING"

# The image the builder produced (and tagged) must exist in the daemon — proof the
# builder loaded it via the projected socket.
echo "==> asserting the built image is present in the daemon"
if ! docker image inspect "$BUILT_IMAGE" >/dev/null 2>&1; then
  echo "!! $BUILT_IMAGE is not in the daemon — the builder did not load it"
  exit 1
fi
echo "    $BUILT_IMAGE present ($(docker image inspect -f '{{.Id}}' "$BUILT_IMAGE"))"
echo "==> builder-container smoke test PASS — the build ran in a builder image distinct from the engine"
