#!/usr/bin/env bash
#
# Build-step smoke test: the OCI language host builds the program image *itself*,
# rather than being handed a prebuilt one. Every other smoke test pre-builds the
# program image on the host and passes its tag via the `image` runtime option;
# here the project declares only a `build` command, and the language host's Run
# executes it (design §7: a shell command that prints an image ref to stdout) to
# produce the image before starting it.
#
# This is faithful to the Option-C model: the engine (and the language host inside
# it) runs in a container with the docker CLI + socket, and the project source is
# mounted in at /project — exactly what the CLI wrapper will provide. The build
# runs against the host daemon over the projected socket and loads the image into
# that same daemon, so the subsequent `docker run` resolves it by ref with no tar
# round-trip.
#
# Reuses the self-contained Node program (its Dockerfile does npm install + COPY,
# needing no host pre-step), so the entire build happens inside the pod.
#
# Pipeline:
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine image
#   2. assemble /project = Node program source + a Pulumi.yaml whose only runtime
#      option is the `build` command (NO prebuilt image)
#   3. run `pulumi up` in the engine container; the language host builds the image
#   4. assert the host built it (log lines + the tagged image landed in the daemon)
#      and the program ran (its stack output)
#
# Usage: run-pod-build.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-build"
PROGRAM_DIR="$SMOKE_DIR/program-node" # self-contained Node program (build needs no host step)
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
BUILT_IMAGE="oci-smoke-build:latest" # what the language host builds in-pod
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
  echo "!! docker daemon not available — cannot run build-step test"
  exit 1
fi

# Start clean so the post-run image-inspect proves THIS run's language host built it.
docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true

echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

echo "==> building engine image $ENGINE_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"

# Assemble the mounted project: the Node program's build context (Dockerfile +
# source) plus the build-only Pulumi.yaml. NO program image is pre-built — the
# language host builds it from this context inside the engine container.
echo "==> assembling /project (program source + build-only Pulumi.yaml — no prebuilt image)"
cp "$PROGRAM_DIR"/* "$WORK/project/"
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

echo "==> running engine container $ENGINE_NAME (the language host builds the program image itself)"
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

echo "==> asserting the language host built the program image and then ran it"
if ! grep -q "oci: building program image" "$WORK/engine.log"; then
  echo "!! the language host did not run the build command"
  exit 1
fi
if ! grep -q "oci: built program image" "$WORK/engine.log"; then
  echo "!! the build command did not produce an image ref"
  exit 1
fi
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ "$GREETING" != "$EXPECTED_GREETING" ]; then
  echo "!! greeting mismatch: got '${GREETING:-<empty>}', want '$EXPECTED_GREETING'"
  exit 1
fi
echo "    greeting = $GREETING"

# The image the language host built (and tagged) must exist in the daemon — proof
# the build ran in-pod, not a pre-baked artifact.
echo "==> asserting the built image is present in the daemon"
if ! docker image inspect "$BUILT_IMAGE" >/dev/null 2>&1; then
  echo "!! $BUILT_IMAGE is not in the daemon — the in-pod build did not load it"
  exit 1
fi
echo "    $BUILT_IMAGE present ($(docker image inspect -f '{{.Id}}' "$BUILT_IMAGE"))"
echo "==> build-step smoke test PASS — the OCI language host built the program image and ran it"
