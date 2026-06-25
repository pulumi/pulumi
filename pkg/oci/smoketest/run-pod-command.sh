#!/usr/bin/env bash
#
# Workspace-coupled-provider smoke test (design Phase 5). Proves the
# run-from-program-image model: a provider that needs the program's filesystem
# and toolchain (here `command`) runs *from the program image* with its binary
# injected, rather than from its own image or via a copied workspace volume.
#
# The program uses command.local.Command to `cat /workspace/marker` — a file
# baked into the PROGRAM image and present in no provider image. For the command
# to succeed, the provider must run rooted in the program's filesystem. The engine
# (in pod mode) sees `command` is workspace-coupled, copies the stock command
# binary out of its provider image into an ephemeral volume, and runs it from the
# program image (PULUMI_POD_PROGRAM_IMAGE) on the engine's netns, then attaches.
#
# Pipeline:
#   1. cross-compile this branch's pulumi + pulumi-language-oci; build the engine
#      image (Dockerfile.cli) and the demo program image (Dockerfile.command,
#      which bakes /workspace/marker)
#   2. download + wrap the stock command provider binary into an image
#   3. create a pod network, run `pulumi up` in the engine container
#   4. the engine runs the command provider FROM the program image and it reads
#      the baked marker
#
# Usage: run-pod-command.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-command"
PROGRAM_DIR="$SMOKE_DIR/program-command"
PKG_DIR="$SMOKE_DIR/../.." # the pkg/ Go module, where the CLI + host live

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder. `docker run`/`network`/`ps` use the default
# context and are unaffected.
BUILDER="${OCI_BUILDER:-desktop-linux}"

GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# The stock provider version is kept in lockstep with the SDK the program builds
# against (program-command/go.mod requires pulumi-command/sdk v1.1.0). The engine's
# container host resolves the image by the same convention:
# pulumi-provider-<name>:v<version>.
PROVIDER_PKG="command"
PROVIDER_VERSION="1.1.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-command:latest"
PROVIDER_IMAGE="pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
EXPECTED_MARKER="hello-from-the-program-workspace"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  # Backstop: remove every container labeled for this pod (engine + any provider
  # container the engine started, which shares the engine's netns) and the pod's
  # volumes. The engine's host.Close() already tears these down on the happy path;
  # this covers a mid-run failure.
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  local vols
  vols="$(docker volume ls -q --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$vols" ] && docker volume rm -f $vols >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run workspace-coupled-provider test"
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

echo "==> cross-compiling demo program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE (bakes /workspace/marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile.command" "$SMOKE_DIR"

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH) and wrapping it"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME on $NET (engine runs command FROM the program image)"
# PULUMI_POD_PROGRAM_IMAGE tells the container host which image workspace-coupled
# providers run from; the command provider is rooted in it and reads its workspace.
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
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE marker=<<%s>>\n" "$(pulumi stack output marker --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the command provider ran from the program image and read the workspace"
if ! grep -q 'oci: provider command is workspace-coupled' "$WORK/engine.log"; then
  echo "!! the engine did not run command from the program image"
  exit 1
fi
MARKER="$(sed -n 's/.*SMOKE marker=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  echo "   (the command provider did not read the program image's baked workspace)"
  exit 1
fi
echo "    marker = $MARKER"
echo "==> workspace-coupled-provider smoke test PASS"
