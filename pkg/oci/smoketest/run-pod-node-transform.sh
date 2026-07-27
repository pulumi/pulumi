#!/usr/bin/env bash
#
# Node twin of run-pod-transform.sh: proves the NODE SDK honors the callback-advertise
# contract end to end, so the engine can dial back into a program running in its own
# network namespace.
#
# Why this test needs more scaffolding than the Go one: the Go program is a static binary
# built against the in-repo SDK by `go build`, so it carries the branch automatically. A
# Node program installs @pulumi/pulumi from the registry, and no RELEASED SDK has the
# advertise behavior — so the image must be given this branch's build or the test would
# silently exercise the wrong code. See program-node-transform/Dockerfile for why the
# branch SDK is layered over the installed package rather than installed as a tarball.
#
# UNLIKE the Go test, this one is NOT a ratchet: the fix already landed, so it passes on
# its first run. A green run therefore cannot, by itself, distinguish "the Node SDK used the
# advertise host" from "it passed for some other reason" — so it was checked against a
# negative control (advertiseHost forced off in sdk/nodejs/runtime/callbacks.ts, SDK rebuilt).
#
# What that control revealed is worth knowing before reading a failure here: Node does NOT
# fail the way Go does. Go surfaces `dial tcp 127.0.0.1:<port>: connection refused` and the
# update errors. Node instead reports the update as SUCCESSFUL, having created only the Stack
# — the RandomPet is silently never registered, and the only symptom is the missing output.
# So a failure of this test looks like "no petName", not like a visible dial error, and the
# absence of an error message is expected rather than a sign of a different problem.
#
# Note also that the "program advertises its callback server at ..." diagnostic comes from
# the LANGUAGE HOST, which is language-agnostic — it proves the env var was SET, not that
# the Node SDK read it. Only the transform actually applying proves the latter.
#
# Usage: run-pod-node-transform.sh
# Requires a running Docker daemon, the repo Go toolchain, and a built sdk/nodejs.
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-node-transform"
PROGRAM_DIR="$SMOKE_DIR/program-node-transform"
PKG_DIR="$SMOKE_DIR/../.."
NODE_SDK_DIR="$SMOKE_DIR/../../../sdk/nodejs"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="nxform-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
PROGRAM_CONTAINER="$NET-program"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-node-transform:latest"
# Tagged with the public-source-qualified name the default resolver computes, so it is a
# local store hit and no registry proxy is needed (see run-pod-provider-address.sh).
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
  rm -rf "$PROGRAM_DIR/sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run node transform test"; exit 1
fi

build_engine_image

# ── stage this branch's Node SDK into the build context ─────────────────────
# Rebuilt every run, not reused: a stale bin/ would ship an SDK without the change and the
# failure would look like the change being wrong rather than the artifact being old.
echo "==> building this branch's Node SDK (sdk/nodejs -> bin/)"
( cd "$NODE_SDK_DIR" && mise exec -- make build_package >/dev/null )
if ! grep -q "PULUMI_CALLBACKS_ADVERTISE_HOST" "$NODE_SDK_DIR/bin/runtime/callbacks.js"; then
  echo "!! the built Node SDK does not contain the advertise-host change — bin/ is stale or the build failed"
  exit 1
fi
if [ ! -f "$NODE_SDK_DIR/bin/cmd/run/index.js" ]; then
  echo "!! the built Node SDK is missing cmd/run, which oci-bootstrap.sh execs"; exit 1
fi
rm -rf "$PROGRAM_DIR/sdk-bin"
cp -R "$NODE_SDK_DIR/bin" "$PROGRAM_DIR/sdk-bin"
echo "   staged $(du -sh "$PROGRAM_DIR/sdk-bin" | cut -f1) of SDK into the build context"

echo "==> building node program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load -q \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$PROGRAM_DIR" >/dev/null

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

# Topology evidence, same as the Go test: capture the program container's netns as it appears.
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
# Plain if/elif: a bare `[ ... ] && echo` here would abort the whole script under `set -e`
# whenever the test is false, killing the run before the verdict below is ever printed.
if [ -z "$NETMODE" ]; then
  echo "    (program container was not caught in time — no NetworkMode recorded)"
elif [ "${NETMODE#container:}" != "$NETMODE" ]; then
  echo "    program container NetworkMode = $NETMODE"
  echo "    -> program SHARES another container's netns (loopback callbacks would resolve anyway)"
else
  echo "    program container NetworkMode = $NETMODE"
  echo "    -> program has its OWN netns on '$NETMODE': loopback would NOT resolve, so the"
  echo "       advertised host is what makes the dial-back possible"
fi

echo "==> checking the transform actually applied"
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
case "$PET" in
  transformed-*)
    echo "    petName = $PET (carries the transform's prefix)"
    # The language host is language-agnostic, so this line proves only that the env var was
    # SET. It is asserted to catch the var being dropped, NOT as evidence about the Node SDK —
    # the prefix above is what shows the SDK consumed it.
    if ! grep -q 'oci: program advertises its callback server at' "$WORK/engine.log"; then
      echo "!! the transform applied, but the language host never advertised a callback host."
      echo "   Something other than the advertise path made this pass — find out what."
      exit 1
    fi
    echo "==> NODE TRANSFORM smoke test PASS — the engine dialed back into a Node program"
    ;;
  "")
    echo "!! no petName output (engine exit $UP_STATUS)"
    echo "   Expect NO error message above: when the Node callback address is unreachable the"
    echo "   update still reports success, having created only the Stack — the resource is"
    echo "   silently skipped. Verified against a negative control; see the header. The cause"
    echo "   is the advertised callback host (sdk/nodejs/runtime/callbacks.ts)."
    echo "   Resources actually created:"
    grep -E "^Resources:|^ *\+ [0-9]+ created|created \(" "$WORK/engine.log" | tail -5 || true
    exit 1
    ;;
  *)
    echo "!! petName = $PET — the resource was created but WITHOUT the transform's prefix,"
    echo "   so the transform silently did not run. A skipped transform is worse than a"
    echo "   failed one: the program's intent was dropped with no error."
    exit 1
    ;;
esac
