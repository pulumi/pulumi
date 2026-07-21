#!/usr/bin/env bash
#
# CRI provider smoke test. Extends the CRI pod-up smoke (run-pod-cri.sh) with a
# real resource provider: the engine starts the random provider as a sibling CRI
# container in the same sandbox, the program creates a RandomPet through it, and
# the test asserts the output. This proves the full provider boot path through
# the CRI pod manager: image pull/existence check, RunContainer for the provider,
# scrapeServingPort from CRI logs, NewProviderAttached over loopback, and the
# workspace volume mount shared between program and provider.
#
# The `random` provider is stateless — it runs from its own image, not the
# program image — so it avoids CopyFromImage (the binary-injection path used by
# `command`). That path is still stubbed and is a separate test.
#
# Prerequisites: same as run-pod-cri.sh (docker, crienv, Go toolchain).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

CRIENV=crienv
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"
PROVIDER_IMAGE="pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_ID="cri-prov-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
STACK="dev"

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

echo "==> cross-compiling random program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

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
printf 'SMOKE petName=<<%s>>\n' \
  "$(pulumi stack output petName --stack "$STACK")"
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
    { "key": "PULUMI_POD_MODE",             "value": "true" },
    { "key": "PULUMI_POD_ADVERTISE_HOST",  "value": "127.0.0.1" },
    { "key": "PULUMI_POD_RUNTIME",        "value": "cri" },
    { "key": "PULUMI_POD_SANDBOX_ID",     "value": "$SB" },
    { "key": "PULUMI_POD_LOG_DIR",        "value": "$LOGDIR" },
    { "key": "PULUMI_POD_ID",             "value": "$POD_ID" },
    { "key": "PULUMI_POD_VOLUME_DIR",     "value": "$VOLDIR" },
    { "key": "PULUMI_POD_PROGRAM_IMAGE",  "value": "$PROGRAM_IMAGE" },
    { "key": "PULUMI_BACKEND_URL",        "value": "file:///state" },
    { "key": "PULUMI_CONFIG_PASSPHRASE",  "value": "smoke-test" },
    { "key": "STACK",                     "value": "$STACK" }
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
  echo "!! CRI provider smoke test FAIL"
  echo ""
  echo "This is expected for the first run — the point is to see WHAT broke."
  exit 1
fi

if ! grep -q 'oci: provider random running as container' "$WORK/engine.log"; then
  echo "!! the engine did not start the provider as a container"
  exit 1
fi

PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the provider did not create the resource"
  exit 1
fi
echo "    petName = $PET"
echo "==> CRI provider smoke test PASS"
