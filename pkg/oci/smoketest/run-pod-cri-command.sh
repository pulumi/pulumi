#!/usr/bin/env bash
#
# CRI command (run-from-program-image) provider smoke test. The `command`
# provider runs from the PROGRAM image — rooted in the program's filesystem so
# it sees the program's toolchain and workspace — with its binary injected from
# the provider image via CopyFromImage. This exercises the CRI manager's
# CopyFromImage (copy-container: a short-lived CRI container that `cp -a`s the
# provider binary into a host-dir volume) and the full inject-then-run path.
#
# The program uses command.local.Command to run `jq` — a binary baked onto the
# PROGRAM image's PATH and present in no provider image. That discriminates
# WHERE the provider ran: the shared workspace mount carries files, not a
# toolchain, so only a provider running from the program image can find jq.
#
# Prerequisites: same as run-pod-cri.sh (docker, crienv, Go toolchain).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-command"
PROGRAM_DIR="$SMOKE_DIR/program-command"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

CRIENV=crienv
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-command:latest"
PROVIDER_PKG="command"
PROVIDER_VERSION="1.1.0"
PROVIDER_IMAGE="pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_ID="cri-cmd-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
STACK="dev"

EXPECTED_TOOLCHAIN="toolchain-from-the-program-image"
EXPECTED_MARKER="hello-from-the-program-workspace"
EXPECTED_CRED="fake-cloud-credential-9f3a"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  echo "== cleanup =="
  if [ -n "${SB:-}" ]; then
    docker exec "$CRIENV" crictl stopp "$SB" >/dev/null 2>&1 || true
    docker exec "$CRIENV" crictl rmp -f "$SB" >/dev/null 2>&1 || true
    echo "   reaped sandbox $SB"
  fi
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

# ── preflight ──────────────────────────────────────────────────────────────
if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available"; exit 1
fi
if ! docker exec "$CRIENV" crictl version >/dev/null 2>&1; then
  echo "!! crienv container not running"; exit 1
fi

# ── build engine + program + provider images ───────────────────────────────
build_engine_image

echo "==> cross-compiling command program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE (bakes jq + /workspace/marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile.command" "$SMOKE_DIR"

echo "==> downloading stock $PROVIDER_PKG provider v$PROVIDER_VERSION (linux/$GOARCH)"
PROVIDER_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$PROVIDER_PKG-v$PROVIDER_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$PROVIDER_URL" -o "$WORK/provider.tar.gz"
tar -xzf "$WORK/provider.tar.gz" -C "$WORK/provctx" "pulumi-resource-$PROVIDER_PKG"
mv "$WORK/provctx/pulumi-resource-$PROVIDER_PKG" "$WORK/provctx/provider-bin"

echo "==> building provider image $PROVIDER_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROVIDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

# ── load all three images into crienv's k8s.io store ───────────────────────
for img in "$ENGINE_IMAGE" "$PROGRAM_IMAGE" "$PROVIDER_IMAGE"; do
  echo "==> loading $img into crienv k8s.io store"
  docker save "$img" | docker exec -i "$CRIENV" ctr -n k8s.io images import -
done

# ── reap any stale sandbox ─────────────────────────────────────────────────
for p in $(docker exec "$CRIENV" crictl pods --name "$POD_ID" -q 2>/dev/null); do
  docker exec "$CRIENV" crictl stopp "$p" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmp -f "$p" >/dev/null 2>&1 || true
done

# ── create the pod sandbox ─────────────────────────────────────────────────
echo "==> creating PodSandbox $POD_ID"
cat > "$WORK/sandbox.json" <<JSON
{
  "metadata": {
    "name": "$POD_ID",
    "namespace": "pulumi",
    "uid": "$POD_ID",
    "attempt": 1
  },
  "log_directory": "$LOGDIR",
  "linux": {
    "security_context": {
      "namespace_options": { "network": 0 }
    }
  }
}
JSON
docker cp "$WORK/sandbox.json" "$CRIENV:/tmp/sandbox.json"
docker exec "$CRIENV" mkdir -p "$LOGDIR" "$VOLDIR"

SB="$(docker exec "$CRIENV" crictl runp /tmp/sandbox.json)"
echo "   sandbox: $SB"

# ── prepare project + state + engine script ────────────────────────────────
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
docker exec "$CRIENV" mkdir -p /cri-smoke/project /cri-smoke/state
docker cp "$WORK/project/Pulumi.yaml" "$CRIENV:/cri-smoke/project/Pulumi.yaml"

cat > "$WORK/engine-run.sh" <<'SCRIPT'
#!/bin/sh
set -e
pulumi login "$PULUMI_BACKEND_URL"
pulumi stack select --create "$STACK"
pulumi up --yes --skip-preview --stack "$STACK"
printf 'SMOKE toolchain=<<%s>> marker=<<%s>> cred=<<%s>> runtimeOutput=<<%s>>\n' \
  "$(pulumi stack output toolchain --stack "$STACK")" \
  "$(pulumi stack output marker --stack "$STACK")" \
  "$(pulumi stack output cred --stack "$STACK")" \
  "$(pulumi stack output runtimeOutput --stack "$STACK")"
SCRIPT
docker cp "$WORK/engine-run.sh" "$CRIENV:/cri-smoke/engine-run.sh"
docker exec "$CRIENV" chmod +x /cri-smoke/engine-run.sh

# ── start the engine as a CRI container in the sandbox ─────────────────────
echo "==> creating engine container in sandbox $SB"
cat > "$WORK/engine-container.json" <<JSON
{
  "metadata": { "name": "engine", "attempt": 0 },
  "image": { "image": "$ENGINE_IMAGE" },
  "command": ["/bin/sh"],
  "args": ["/cri-smoke/engine-run.sh"],
  "working_dir": "/project",
  "envs": [
    { "key": "PULUMI_POD_MODE",          "value": "true" },
    { "key": "PULUMI_POD_RUNTIME",        "value": "cri" },
    { "key": "PULUMI_POD_SANDBOX_ID",     "value": "$SB" },
    { "key": "PULUMI_POD_LOG_DIR",        "value": "$LOGDIR" },
    { "key": "PULUMI_POD_ID",             "value": "$POD_ID" },
    { "key": "PULUMI_POD_VOLUME_DIR",     "value": "$VOLDIR" },
    { "key": "PULUMI_POD_PROGRAM_IMAGE",  "value": "$PROGRAM_IMAGE" },
    { "key": "PULUMI_BACKEND_URL",        "value": "file:///state" },
    { "key": "PULUMI_CONFIG_PASSPHRASE",  "value": "smoke-test" },
    { "key": "STACK",                     "value": "$STACK" },
    { "key": "OCI_SMOKE_FAKE_CRED",       "value": "$EXPECTED_CRED" }
  ],
  "mounts": [
    { "host_path": "/run/containerd/containerd.sock", "container_path": "/run/containerd/containerd.sock" },
    { "host_path": "$LOGDIR",       "container_path": "$LOGDIR" },
    { "host_path": "$VOLDIR",       "container_path": "$VOLDIR" },
    { "host_path": "/cri-smoke/project", "container_path": "/project" },
    { "host_path": "/cri-smoke/state",   "container_path": "/state" },
    { "host_path": "/cri-smoke",         "container_path": "/cri-smoke" }
  ],
  "log_path": "engine_0.log"
}
JSON
docker cp "$WORK/engine-container.json" "$CRIENV:/tmp/engine-container.json"

ENGINE_ID="$(docker exec "$CRIENV" crictl create "$SB" /tmp/engine-container.json /tmp/sandbox.json)"
echo "   engine container: $ENGINE_ID"

echo "==> starting engine container"
docker exec "$CRIENV" crictl start "$ENGINE_ID"

# ── wait for the engine to finish ──────────────────────────────────────────
echo "==> waiting for engine container to exit..."
TIMEOUT=180
ELAPSED=0
while true; do
  STATE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.state}}' "$ENGINE_ID" 2>/dev/null || echo "unknown")"
  if [ "$STATE" = "CONTAINER_EXITED" ]; then
    break
  fi
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "!! engine container did not exit within ${TIMEOUT}s"
    echo "== engine logs =="
    docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 || true
    exit 1
  fi
  sleep 2
  ELAPSED=$((ELAPSED + 2))
done

EXIT_CODE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.exitCode}}' "$ENGINE_ID" 2>/dev/null || echo "-1")"
echo "==> engine exited with code $EXIT_CODE after ~${ELAPSED}s"

# ── capture and display logs ──────────────────────────────────────────────
echo "== engine logs =="
docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 | tee "$WORK/engine.log"

# ── assertions ─────────────────────────────────────────────────────────────
echo "==> checking results"

if [ "$EXIT_CODE" != "0" ]; then
  echo "!! engine exited with code $EXIT_CODE (expected 0)"
  echo "!! CRI command provider smoke test FAIL"
  echo ""
  echo "This is expected for the first run — the point is to see WHAT broke."
  exit 1
fi

if ! grep -q "oci: provider command needs the program's toolchain" "$WORK/engine.log"; then
  echo "!! the engine did not run command from the program image"
  exit 1
fi

TOOLCHAIN="$(sed -n 's/.*SMOKE toolchain=<<\(.*\)>> marker=.*/\1/p' "$WORK/engine.log" | head -1)"
MARKER="$(sed -n 's/.*marker=<<\(.*\)>> cred=.*/\1/p' "$WORK/engine.log" | head -1)"
CRED="$(sed -n 's/.*cred=<<\(.*\)>> runtimeOutput=.*/\1/p' "$WORK/engine.log" | head -1)"
RUNTIME_OUTPUT="$(sed -n 's/.*runtimeOutput=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"

if [ "$TOOLCHAIN" != "$EXPECTED_TOOLCHAIN" ]; then
  echo "!! toolchain mismatch: got '${TOOLCHAIN:-<empty>}', want '$EXPECTED_TOOLCHAIN'"
  echo "   (the command provider did not get the program image's PATH — jq not found?)"
  exit 1
fi
echo "    toolchain = $TOOLCHAIN (jq ran from the program image's PATH)"

if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  exit 1
fi
echo "    marker = $MARKER (workspace seed from the image)"

if [ "$CRED" != "$EXPECTED_CRED" ]; then
  echo "!! cred mismatch: got '${CRED:-<empty>}', want '$EXPECTED_CRED'"
  echo "   (the engine's env was not projected onto the command provider container)"
  exit 1
fi
echo "    cred = $CRED (projected from engine env)"

EXPECTED_RUNTIME_OUTPUT="written-at-runtime"
if [ "$RUNTIME_OUTPUT" != "$EXPECTED_RUNTIME_OUTPUT" ]; then
  echo "!! runtimeOutput mismatch: got '${RUNTIME_OUTPUT:-<empty>}', want '$EXPECTED_RUNTIME_OUTPUT'"
  echo "   (the provider did not read a file the program wrote at runtime — volume sharing broken)"
  exit 1
fi
echo "    runtimeOutput = $RUNTIME_OUTPUT (live volume sharing: program wrote, provider read)"
echo "==> CRI command provider smoke test PASS"
