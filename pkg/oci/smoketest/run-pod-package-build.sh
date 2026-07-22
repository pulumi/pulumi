#!/usr/bin/env bash
#
# `pulumi package build` smoke test — the source→image seam (the dev-time twin of the
# registry proxy, which wraps released *binaries* as images; this builds local *source*
# into images). A package describes itself in PulumiPlugin.yaml (runtime: oci +
# options.{name, version, build}); `package build` runs the build in a builder container
# whose image supplies the toolchain, tagging the result by the provider convention
# (pulumi/pulumi-provider-<name>:v<version>) and leaving it in the local store. This is the
# principled home for "where does the build run?" — the same builder-container mechanism
# the language host already uses for local components, now reachable as a command.
#
# Discriminating proof (vs. a no-op that any cached image would pass):
#   1. the greeting image is REMOVED before the run, so it must be freshly built;
#   2. the build command guards on /only-in-builder — a marker baked into the builder
#      image ALONE — so a build routed anywhere but the builder container fails the
#      `test -e` and produces nothing. So the image being present afterwards, under the
#      exact convention ref, proves the build ran in the builder and tagged it correctly.
#
# (Build-then-consume — publishing this built image and Constructing it from a pinned
# `package add` ref — is proven separately by run-pod-publish.sh.)
#
# Usage: run-pod-package-build.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
COMPONENT_DIR="$SMOKE_DIR/component-greeter"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

COMPONENT_PKG="greeting"
COMPONENT_VERSION="0.1.0"

POD_ID="smoke-$$"
ENGINE_NAME="pulumi-pod-$POD_ID-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
BUILDER_IMAGE="oci-smoke-builder:latest" # discriminating builder (carries /only-in-builder)
POD_LABEL="com.pulumi.pod=$POD_ID"
# The convention ref `package build` must produce (no registry configured -> bare ref).
COMPONENT_IMAGE="pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"

WORK="$(mktemp -d)"
mkdir -p "$WORK/cli" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker image rm -f "$COMPONENT_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run package-build test"
  exit 1
fi

# Remove the component image so the run must build it — the unambiguous proof that
# `package build` (not a prebuild/cache) produced it.
docker image rm -f "$COMPONENT_IMAGE" >/dev/null 2>&1 || true

build_engine_image

echo "==> building builder image $BUILDER_IMAGE (docker CLI + /only-in-builder marker the engine lacks)"
docker buildx build --builder "$BUILDER" --load \
  -t "$BUILDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.builder" "$SMOKE_DIR"

# The package's source (incl. its self-describing PulumiPlugin.yaml) goes under the
# mounted workspace, where the builder reaches it via --volumes-from the engine.
mkdir -p "$WORK/project/component-greeter"
cp "$COMPONENT_DIR"/* "$WORK/project/component-greeter/"

echo "==> pulumi package build component-greeter (build local source -> plugin image, in the builder)"
# Run inside the engine container (pod mode): package build needs --volumes-from the
# engine to reach the source (no engine netns — unlike the schema fetch). The docker
# socket lets the builder load the image into the shared daemon.
docker run --rm -i \
  --privileged \
  --name "$ENGINE_NAME" \
  --hostname "$ENGINE_NAME" \
  --label "$POD_LABEL" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$WORK/project":/project \
  -w /project \
  -e PULUMI_POD_MODE=true \
  -e PULUMI_POD_ID="$POD_ID" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c 'pulumi package build component-greeter' \
  2>&1 | tee "$WORK/build.log"

echo "==> asserting package build printed the convention ref"
if ! grep -qx "$COMPONENT_IMAGE" "$WORK/build.log"; then
  echo "!! package build did not print the expected ref $COMPONENT_IMAGE on stdout"
  exit 1
fi
echo "    printed ref: $COMPONENT_IMAGE"

echo "==> asserting the build ran in the builder container (toolchain image, not the engine)"
if ! grep -q "Building $COMPONENT_PKG .*in $BUILDER_IMAGE" "$WORK/build.log"; then
  echo "!! the build did not run in the builder image"
  exit 1
fi

echo "==> asserting the freshly-built plugin image is present under the convention ref"
# The build command guards on /only-in-builder, so this image existing proves the build
# ran in the builder (a build anywhere else would have failed the guard and produced nothing).
if ! docker image inspect "$COMPONENT_IMAGE" >/dev/null 2>&1; then
  echo "!! $COMPONENT_IMAGE is not in the daemon — package build did not produce it"
  exit 1
fi
echo "    $COMPONENT_IMAGE present ($(docker image inspect -f '{{.Id}}' "$COMPONENT_IMAGE"))"
echo "==> package build smoke test PASS — local source was built into a conventionally-named"
echo "    plugin image, in a builder container, with nothing pre-built"
