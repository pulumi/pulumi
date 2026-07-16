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
# (pulumi/pulumi-provider-<name>:v<version>). The built image lands in the shared docker
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
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
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
# of the stock `random` provider (proven by run-pod-mlc.sh) — pulled through the
# wrapper's shared registry proxy, which synthesizes it from the released binary.
RANDOM_PKG="random"
RANDOM_VERSION="4.21.0"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
BUILDER_IMAGE="oci-smoke-builder:latest" # discriminating builder for the component build
PROGRAM_IMAGE="oci-smoke-mlc:latest"
# The wrapper points the engine at its shared registry proxy, so the in-pod build
# tags the component registry-qualified (and org-namespaced) — the same ref the
# engine resolves at Construct time.
PROXY_HOST="localhost:${PULUMI_POD_PROXY_PORT:-5005}"
COMPONENT_IMAGE="$PROXY_HOST/pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION" # built in-pod, not prebuilt
STACK="dev"
EXPECTED_FRAGMENT="from a Node multi-language component"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod itself; this clears the in-pod-built component
  # image, the cross-compiled binary, and the scratch dir.
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

build_engine_image

echo "==> building builder image $BUILDER_IMAGE (docker CLI + /only-in-builder marker the engine lacks)"
docker buildx build --builder "$BUILDER" --load \
  -t "$BUILDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.builder" "$SMOKE_DIR"

echo "==> cross-compiling Go consumer program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building Go consumer image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

# NOTE: the greeting component image is deliberately NOT built here — the language
# host builds it during InstallDependencies from the mounted source.

# NOTE: the random provider image is deliberately NOT built here either — the
# wrapper's shared registry proxy synthesizes it from the released binary when the
# recursive child creation pulls it.

# Assemble the mounted dir: the program's Pulumi.yaml (declaring the local
# component) plus the component's source under the declared `path`, so
# InstallDependencies can build it.
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
mkdir -p "$WORK/project/component-greeter"
cp "$COMPONENT_DIR"/* "$WORK/project/component-greeter/"

export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"

# install and up run as *separate* pods: the component image `pulumi-pod install`
# builds lands in the shared daemon, so the up pod resolves it at Construct time.
# This is the cross-pod daemon-as-artifact-store the design relies on, now driven
# by plain pulumi-pod commands.
echo "==> pulumi-pod: stack init, install (builds the local component), up, output"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" install 2>&1 | tee "$WORK/install.log"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"
MESSAGE="$("$WRAPPER" stack output message)"

echo "==> asserting install built the local component via the shared package-build mechanism, in the builder"
# install delegates to oci.BuildPackage (the same path as `pulumi package build`), which
# logs "Building <name> (...) in <builder> -> <ref>". The build itself guards on a marker
# only the builder image carries, so an in-process build would have failed rather than
# reach this assertion.
if ! grep -q "Building $COMPONENT_PKG .*in $BUILDER_IMAGE" "$WORK/install.log"; then
  echo "!! install did not build the local component in the builder container"
  exit 1
fi
if ! docker image inspect "$COMPONENT_IMAGE" >/dev/null 2>&1; then
  echo "!! $COMPONENT_IMAGE is not in the daemon — the in-pod component build did not produce it"
  exit 1
fi
echo "    $COMPONENT_IMAGE present ($(docker image inspect -f '{{.Id}}' "$COMPONENT_IMAGE"))"

echo "==> asserting the engine started the (locally built) component and Construct returned its output"
if ! grep -q "oci: provider $COMPONENT_PKG running as container" "$WORK/up.log"; then
  echo "!! the engine did not start the component as a container"
  exit 1
fi
if ! grep -q "oci: provider $RANDOM_PKG running as container" "$WORK/up.log"; then
  echo "!! the component did not recursively start the random provider as a container"
  exit 1
fi
if ! grep -qE "random:index:RandomPet .*created" "$WORK/up.log"; then
  echo "!! the component's RandomPet child was not created"
  exit 1
fi

case "$MESSAGE" in
  *"$EXPECTED_FRAGMENT"*"(pet: "*) echo "    message = $MESSAGE" ;;
  *) echo "!! component output missing child pet name or unexpected: '${MESSAGE:-<empty>}'"; exit 1 ;;
esac
echo "==> local-component build smoke test PASS — the OCI language host built the MLC image,"
echo "    and a Go program drove that locally-built Node component end-to-end"
