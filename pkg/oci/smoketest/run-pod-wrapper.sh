#!/usr/bin/env bash
#
# pulumi-pod wrapper smoke test. Where the other tests hand-roll the engine
# invocation (a ~30-line `docker run --privileged --network ... -e PULUMI_POD_*
# ... --entrypoint sh ... pulumi up` block), this drives the deployment with plain
# `pulumi-pod` commands — exactly as a user would once the wrapper is on PATH. It
# proves the wrapper bootstraps the pod, sets the PULUMI_POD_* contract, mounts the
# project + state, and tears down — and that each invocation is a self-contained
# pod that shares state through the mounted backend.
#
# Reuses the random program (a stateless containerized provider). The only build
# steps left in the test are the branch-specific image builds (engine + program);
# all orchestration is the wrapper's job.
#
# Usage: run-pod-wrapper.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
WRAPPER="$SMOKE_DIR/pulumi-pod"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod itself; this only clears anything a crashed run
  # left behind and the scratch dir.
  local leftovers
  leftovers="$(docker ps -aq --filter "label=com.pulumi.pod" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run wrapper test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling + building program image $PROGRAM_IMAGE"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

echo "==> wrapping stock $PROVIDER_PKG provider v$PROVIDER_VERSION"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Everything below is what a user would actually type. The wrapper owns the pod —
# and, with no state/home/backend env set, defaults PULUMI_HOME *and* the file
# backend into the mounted dir (.pulumi-pod). So login, stack selection, and state
# persist across these independent pods with zero extra config — which is exactly
# what the no---stack commands below exercise.
export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"

echo "==> pulumi-pod stack init $STACK (selection persists via mounted PULUMI_HOME)"
"$WRAPPER" stack init "$STACK"

echo "==> pulumi-pod up (no --stack: uses the persisted stack selection)"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"

echo "==> pulumi-pod stack output petName (no --stack)"
PET="$("$WRAPPER" stack output petName)"

echo "==> asserting the deployment ran through the wrapper-bootstrapped pod"
if ! grep -q "oci: provider $PROVIDER_PKG running as container" "$WORK/up.log"; then
  echo "!! the engine did not start the provider as a container under the wrapper"
  exit 1
fi
if [ -z "$PET" ]; then
  echo "!! no petName output — the deployment did not create the resource"
  exit 1
fi
echo "    petName = $PET"
echo "==> pulumi-pod wrapper smoke test PASS — a deployment ran via plain pulumi-pod commands"
