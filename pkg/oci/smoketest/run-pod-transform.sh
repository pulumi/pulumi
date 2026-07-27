#!/usr/bin/env bash
#
# Resource-transform smoke test — a probe of program<->engine topology.
#
# Everything else in the pod dials OUT: the program dials the monitor, providers are dialed
# BY the engine (which is why they netns-join). A resource transform is the exception. The
# program's SDK stands up a CALLBACK server and hands the engine its address, so the engine
# dials back INTO the program for every resource registration.
#
# That callback server binds 127.0.0.1 on a kernel-chosen port and advertises that literal
# address (sdk/go/pulumi/callback.go builds `Target: "127.0.0.1:" + port`). It is therefore
# reachable only where the engine shares the program's loopback — and whether it does is
# RUNTIME-DEPENDENT today:
#
#   docker/nerdctl — MEASURED: the program runs in its OWN netns as a sibling on the pod
#                    bridge and reaches the engine by advertised DNS name (podAdvertiseHost;
#                    see pulumi-language-oci). The engine dialing 127.0.0.1:<port> reaches
#                    ITSELF, not the program, and the update fails with
#                    "invoke transform: ... dial tcp 127.0.0.1:<port>: connection refused".
#   CRI            — INFERRED, UNVERIFIED: every pod member shares the one sandbox netns, so
#                    loopback should be shared and the callback should resolve. Nobody has
#                    run this on CRI; do not treat that path as covered.
#
# This script runs the DOCKER path, so it is EXPECTED TO FAIL TODAY. It is written as a
# ratchet, not a characterization test: it asserts the CORRECT behavior (the transform is
# applied), so it turns green on its own once the program's callback server can bind an
# addressable endpoint — the same plugin bind contract that lets providers be attached by
# DNS (PULUMI_PLUGIN_LISTEN_ADDRESS). Do not "fix" it by asserting the failure.
#
# It also records the program container's NetworkMode, so a run is evidence about the
# topology rather than an argument about it.
#
# Usage: run-pod-transform.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-transform"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="xform-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
PROGRAM_CONTAINER="$NET-program" # pod-namespaced name the pod manager gives the program
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
# Tag the local provider image with the public-source-qualified name the default resolver
# computes, so it is a local store hit and no registry proxy is needed (same trick as
# run-pod-provider-address.sh; see the DefaultPublicRegistry note there).
PROVIDER_IMAGE="pulumi.registry.internal/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  [ -n "${WATCH_PID:-}" ] && kill "$WATCH_PID" >/dev/null 2>&1 || true
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  local vols
  vols="$(docker volume ls -q --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$vols" ] && docker volume rm $vols >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run transform test"; exit 1
fi

build_engine_image

echo "==> cross-compiling transform program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load -q \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR" >/dev/null

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH)"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load -q \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx" >/dev/null

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Capture the program container's netns as it appears — the topology evidence. The program
# is short-lived, so poll for it rather than inspecting after the fact.
( for _ in $(seq 1 600); do
    mode="$(docker inspect -f '{{.HostConfig.NetworkMode}}' "$PROGRAM_CONTAINER" 2>/dev/null || true)"
    if [ -n "$mode" ]; then echo "$mode" >"$WORK/program-netmode"; break; fi
    sleep 0.1
  done ) &
WATCH_PID=$!

echo "==> running engine container $ENGINE_NAME on $NET"
set +e
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
    printf "SMOKE petName=<<%s>>\n" "$(pulumi stack output petName --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"
UP_STATUS=${PIPESTATUS[0]}
set -e

echo "==> topology evidence"
NETMODE="$(cat "$WORK/program-netmode" 2>/dev/null || true)"
if [ -n "$NETMODE" ]; then
  echo "    program container NetworkMode = $NETMODE"
  case "$NETMODE" in
    container:*) echo "    -> program SHARES another container's netns (loopback callbacks would resolve)" ;;
    *)           echo "    -> program has its OWN netns on '$NETMODE' (loopback callbacks cannot resolve)" ;;
  esac
else
  echo "    (program container was not caught in time — no NetworkMode recorded)"
fi

echo "==> checking the transform actually applied"
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
case "$PET" in
  transformed-*)
    echo "    petName = $PET (carries the transform's prefix)"
    echo "==> TRANSFORM smoke test PASS — the engine dialed back into the program"
    ;;
  "")
    echo "!! no petName output — the update did not complete (exit $UP_STATUS)"
    echo "   This is the EXPECTED failure on docker today: the program's callback server"
    echo "   binds 127.0.0.1 in its own netns and advertises that address to the engine."
    grep -iE "callback|transform|connection refused|Unavailable|127\.0\.0\.1" "$WORK/engine.log" | tail -8 || true
    exit 1
    ;;
  *)
    echo "!! petName = $PET — the resource was created but WITHOUT the transform's prefix,"
    echo "   so the transform silently did not run. A skipped transform is worse than a"
    echo "   failed one: the program's intent was dropped with no error."
    exit 1
    ;;
esac
