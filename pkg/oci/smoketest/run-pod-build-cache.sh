#!/usr/bin/env bash
#
# Build-cache smoke test: a build cache volume (build.caches) persists across builds
# and pods, so a second build reuses what the first wrote.
#
# Without persistent caches every build starts from scratch — the nix store, the
# go/npm cache, docker layers all evaporate between ephemeral builder containers (the
# same ephemeral-filesystem failure mode the buildkit-builder leak exposed).
# build.caches mounts a stable, persistent named volume into the builder; this proves
# it actually persists.
#
# The proof is two consecutive builds in *separate* pods: the build records a marker in
# the cache volume and reports OCI-CACHE-MISS (empty cache) the first time and
# OCI-CACHE-HIT (volume reused) the second. If the cache did not persist, the second
# build would MISS too. The marker dance goes to stderr; stdout stays the image ref.
#
# Usage: run-pod-build-cache.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-build-cache"
PROGRAM_DIR="$SMOKE_DIR/program-node" # self-contained Node program (build needs no host step)
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
BUILDER_IMAGE="oci-smoke-builder:latest"
BUILT_IMAGE="oci-smoke-buildcache:latest"     # what the build produces
CACHE_VOL="pulumi-oci-buildcache-buildcache"  # cacheVolumeName("/buildcache")
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/project"

cleanup() {
  # The cache volume is persistent by design (not pod-scoped) — remove it explicitly.
  docker volume rm -f "$CACHE_VOL" >/dev/null 2>&1 || true
  docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run build-cache test"
  exit 1
fi

# Clean slate: remove the cache volume so the first build is a guaranteed MISS.
docker volume rm -f "$CACHE_VOL" >/dev/null 2>&1 || true
docker image rm -f "$BUILT_IMAGE" >/dev/null 2>&1 || true

build_engine_image

echo "==> building builder image $BUILDER_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$BUILDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.builder" "$SMOKE_DIR"

echo "==> assembling /project (Node program + cache-build Pulumi.yaml)"
cp "$PROGRAM_DIR"/* "$WORK/project/"
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"

# Read the build counter straight from the cache volume — ground truth, vs. parsing
# Pulumi's noisy, triple-rendered, interleaved build output.
count_builds() {
  docker run --rm -v "$CACHE_VOL":/c alpine sh -c 'cat /c/n 2>/dev/null || echo 0' | tr -dc '0-9'
}

echo "==> pulumi-pod: stack init, then two ups (separate pods) to exercise the cache"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up1.log"
N1="$(count_builds)"; N1="${N1:-0}"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up2.log"
N2="$(count_builds)"; N2="${N2:-0}"

echo "==> the build increments a counter in the cache volume; it must grow across pods"
echo "    build counter: after pod 1 = $N1, after pod 2 = $N2"
if [ "$N1" -lt 1 ]; then
  echo "!! pod 1 recorded no build in the cache volume (expected >= 1)"
  exit 1
fi
if [ "$N2" -le "$N1" ]; then
  echo "!! counter did not grow across pods ($N2 <= $N1) — the cache content did not persist"
  exit 1
fi
echo "    counter grew $N1 -> $N2 across separate pods — the cache volume persisted and was reused"

echo "==> asserting the cache volume persisted in the daemon"
if ! docker volume inspect "$CACHE_VOL" >/dev/null 2>&1; then
  echo "!! $CACHE_VOL not present — the cache volume was not created/persisted"
  exit 1
fi
echo "    $CACHE_VOL present"
echo "==> build-cache smoke test PASS — build.caches persists across builds and pods"
