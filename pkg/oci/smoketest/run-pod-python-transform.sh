#!/usr/bin/env bash
#
# Python twin of run-pod-node-transform.sh: proves the PYTHON SDK honors the callback-advertise
# contract end to end, so the engine can dial back into a program running in its own network
# namespace.
#
# Like the Node test, the image must be given THIS BRANCH's SDK: a Python program installs
# `pulumi` from the index, and no released version has the advertise behavior, so an
# unmodified image would silently exercise the wrong code. Python needs no build step for
# this — lib/pulumi is the shipped package, generated protobuf modules included — so the
# source tree is copied over the installed one (see the Dockerfile for why overwriting beats
# installing the local tree as a package).
#
# NOT a ratchet: the fix already landed, so this passes on its first run. The Node twin was
# checked against a negative control, which established how this class of failure presents:
# the update reports SUCCESS having created only the Stack, and the resource is silently never
# registered. Expect no error message on failure — just a missing output.
#
# Usage: run-pod-python-transform.sh
# Requires a running Docker daemon and the repo Go toolchain.
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-python-transform"
PROGRAM_DIR="$SMOKE_DIR/program-python-transform"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
PY_SDK_LIB="$REPO_ROOT/sdk/python/lib/pulumi"
EXEC_SHIM="$REPO_ROOT/sdk/python/cmd/pulumi-language-python-exec"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="pxform-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
PROGRAM_CONTAINER="$NET-program"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-python-transform:latest"
# Tagged with the public-source-qualified name the default resolver computes, so it is a local
# store hit and no registry proxy is needed (see run-pod-provider-address.sh).
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
  rm -rf "$PROGRAM_DIR/sdk-lib" "$PROGRAM_DIR/pulumi-language-python-exec"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run python transform test"; exit 1
fi

build_engine_image

# ── stage this branch's Python SDK into the build context ───────────────────
echo "==> staging this branch's Python SDK (sdk/python/lib/pulumi)"
if ! grep -q "PULUMI_CALLBACKS_ADVERTISE_HOST" "$PY_SDK_LIB/runtime/_callbacks.py"; then
  echo "!! the branch Python SDK does not contain the advertise-host change —"
  echo "   $PY_SDK_LIB/runtime/_callbacks.py is not the version under test"
  exit 1
fi
rm -rf "$PROGRAM_DIR/sdk-lib"
cp -R "$PY_SDK_LIB" "$PROGRAM_DIR/sdk-lib"
# Drop compiled caches so the image never runs a .pyc built against a different source.
find "$PROGRAM_DIR/sdk-lib" -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null || true
cp "$EXEC_SHIM" "$PROGRAM_DIR/pulumi-language-python-exec"
echo "   staged $(du -sh "$PROGRAM_DIR/sdk-lib" | cut -f1) of SDK into the build context"

echo "==> building python program image $PROGRAM_IMAGE"
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

# Topology evidence: capture the program container's netns as it appears.
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
# Plain if/elif: a bare `[ ... ] && echo` here would abort the script under `set -e` whenever
# the test is false, killing the run before the verdict below is printed.
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
    # SET. Asserted to catch the var being dropped, NOT as evidence about the Python SDK —
    # the prefix above is what shows the SDK consumed it.
    if ! grep -q 'oci: program advertises its callback server at' "$WORK/engine.log"; then
      echo "!! the transform applied, but the language host never advertised a callback host."
      echo "   Something other than the advertise path made this pass — find out what."
      exit 1
    fi
    echo "==> PYTHON TRANSFORM smoke test PASS — the engine dialed back into a Python program"
    ;;
  "")
    echo "!! no petName output (engine exit $UP_STATUS)"
    echo "   Expect NO error message above: when the callback address is unreachable the update"
    echo "   still reports success, having created only the Stack — the resource is silently"
    echo "   skipped (established by the Node twin's negative control). The cause is the"
    echo "   advertised callback host (sdk/python/lib/pulumi/runtime/_callbacks.py)."
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
