#!/usr/bin/env bash
#
# Local-component build smoke test: the OCI language host builds an MLC's image
# *itself*, rather than the image being prebuilt/published. run-pod-mlc.sh proves
# the *published* MLC case (its image pre-exists, resolved by convention); this
# proves the *local* case — a program-as-component whose source lives alongside the
# program and must be built.
#
# The mechanism: the program declares the greeting component as local in its
# runtime options (a `components:` block). The language host's InstallDependencies
# builds each declared component, tagging it by the provider convention
# (pulumi-provider-<name>:v<version>). The built image lands in the shared docker
# daemon, so the container host (a *different* process — engine vs. pre-install
# host) finds it by tag at Construct time. The daemon is the shared artifact store
# that crosses the process split; no in-process ref handoff is needed.
#
# To make the proof unambiguous we REMOVE the greeting image before the run: if the
# host did not build it, the container host would fail to `docker run` a missing
# tag (the literal "it does not just work without this" state). The run then shows
# the host building it, and the full MLC chain (Construct -> RandomPet child ->
# recursive random provider start) working.
#
# Usage: run-pod-mlc-localbuild.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-mlc-localbuild"
PROGRAM_DIR="$SMOKE_DIR/program-mlc"
COMPONENT_DIR="$SMOKE_DIR/component-greeter"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# The local component. The program pins this version; the language host builds the
# image and tags it by the same convention the container host resolves.
COMPONENT_PKG="greeting"
COMPONENT_VERSION="0.1.0"

# The component registers a random.RandomPet child, which drives a recursive start
# of the stock `random` provider (proven by run-pod-mlc.sh). Wrapped as an image
# here so it is present when the child is created.
RANDOM_PKG="random"
RANDOM_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-mlc:latest"
COMPONENT_IMAGE="pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION" # built in-pod, not prebuilt
RANDOM_IMAGE="pulumi-provider-$RANDOM_PKG:v$RANDOM_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
EXPECTED_FRAGMENT="from a Node multi-language component"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$COMPONENT_IMAGE" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run local-component build test"
  exit 1
fi

# Remove the component image so the run must build it — the unambiguous proof that
# the language host (not a prebuild) produced it.
docker image rm -f "$COMPONENT_IMAGE" >/dev/null 2>&1 || true

echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

echo "==> building engine image $ENGINE_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"

echo "==> cross-compiling Go consumer program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building Go consumer image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

# NOTE: the greeting component image is deliberately NOT built here — the language
# host builds it during InstallDependencies from the mounted source.

echo "==> downloading stock $RANDOM_PKG provider v$RANDOM_VERSION (linux/$GOARCH) and wrapping it"
RANDOM_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$RANDOM_PKG-v$RANDOM_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$RANDOM_URL" -o "$WORK/random.tar.gz"
tar -xzf "$WORK/random.tar.gz" -C "$WORK/provctx" "pulumi-resource-$RANDOM_PKG"
mv "$WORK/provctx/pulumi-resource-$RANDOM_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$RANDOM_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

# Assemble /project: the program's Pulumi.yaml (declaring the local component) plus
# the component's source under the declared `path`, so InstallDependencies can build
# it inside the engine container.
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
mkdir -p "$WORK/project/component-greeter"
cp "$COMPONENT_DIR"/* "$WORK/project/component-greeter/"

echo "==> running engine container $ENGINE_NAME (language host builds the local component, then the chain runs)"
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
  -e PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE" \
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  -e STACK="$STACK" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select --create "$STACK"
    pulumi install
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE message=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the language host built the local component image"
if ! grep -q "oci: building local component $COMPONENT_PKG" "$WORK/engine.log"; then
  echo "!! the language host did not build the local component"
  exit 1
fi
if ! docker image inspect "$COMPONENT_IMAGE" >/dev/null 2>&1; then
  echo "!! $COMPONENT_IMAGE is not in the daemon — the in-pod component build did not produce it"
  exit 1
fi
echo "    $COMPONENT_IMAGE present ($(docker image inspect -f '{{.Id}}' "$COMPONENT_IMAGE"))"

echo "==> asserting the engine started the (locally built) component and Construct returned its output"
if ! grep -q "oci: provider $COMPONENT_PKG running as container" "$WORK/engine.log"; then
  echo "!! the engine did not start the component as a container"
  exit 1
fi
if ! grep -q "oci: provider $RANDOM_PKG running as container" "$WORK/engine.log"; then
  echo "!! the component did not recursively start the random provider as a container"
  exit 1
fi
if ! grep -qE "random:index:RandomPet .*created" "$WORK/engine.log"; then
  echo "!! the component's RandomPet child was not created"
  exit 1
fi

MESSAGE="$(sed -n 's/.*SMOKE message=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
case "$MESSAGE" in
  *"$EXPECTED_FRAGMENT"*"(pet: "*) echo "    message = $MESSAGE" ;;
  *) echo "!! component output missing child pet name or unexpected: '${MESSAGE:-<empty>}'"; exit 1 ;;
esac
echo "==> local-component build smoke test PASS — the OCI language host built the MLC image,"
echo "    and a Go program drove that locally-built Node component end-to-end"
