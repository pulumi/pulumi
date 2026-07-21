#!/usr/bin/env bash
#
# CRI pod-up smoke test. Runs the engine inside a CRI PodSandbox (containerd
# via the CRI plugin) rather than a docker bridge — the first time a real
# `pulumi up` flows through the criPodManager. The engine is a CRI container
# in the sandbox, and the in-engine language host starts the program container
# as a sibling in the same sandbox over the CRI gRPC socket. Everything reaches
# everything on loopback (shared sandbox netns).
#
# This is an e2e composition of two things already proven separately:
#   - Gate 1 (FINDINGS): engine-in-sandbox topology, socket-only run path
#   - TestCriLiveRunWaitLogs: in-process gRPC client against live containerd
# What's new here is a REAL pulumi up flowing through it.
#
# Prerequisites:
#   - docker daemon (to build images and drive crienv)
#   - the `crienv` container (kubelet-free containerd+CRI; see crienv-init.sh
#     in the CRI spike kit ~/scratch/2026-07-18_cri-podmanager/)
#   - the repo Go toolchain (to cross-compile)
#
# Usage: run-pod-cri.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project"
PROGRAM_DIR="$SMOKE_DIR/program"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

CRIENV=crienv
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-demo:latest"
POD_ID="cri-smoke-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
STACK="dev"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  echo "== cleanup =="
  # Reap the sandbox (and everything in it).
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
  echo "!! crienv container not running or crictl not functional — see the CRI spike kit"; exit 1
fi

# ── build ──────────────────────────────────────────────────────────────────
build_engine_image

echo "==> cross-compiling demo program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

# ── load images into crienv's k8s.io store ─────────────────────────────────
# CRI containers see only the k8s.io containerd namespace. Local docker images
# must be exported and imported there; PullImage at runtime also lands in k8s.io.
echo "==> loading $ENGINE_IMAGE into crienv k8s.io store"
docker save "$ENGINE_IMAGE" | docker exec -i "$CRIENV" ctr -n k8s.io images import -

echo "==> loading $PROGRAM_IMAGE into crienv k8s.io store"
docker save "$PROGRAM_IMAGE" | docker exec -i "$CRIENV" ctr -n k8s.io images import -

# ── reap any stale sandbox ─────────────────────────────────────────────────
for p in $(docker exec "$CRIENV" crictl pods --name "$POD_ID" -q 2>/dev/null); do
  docker exec "$CRIENV" crictl stopp "$p" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmp -f "$p" >/dev/null 2>&1 || true
  echo "   reaped stale sandbox $p"
done

# ── create the pod sandbox ─────────────────────────────────────────────────
# The sandbox config is what the wrapper would produce: a PodSandbox with a
# shared network namespace (the topology that makes loopback work), a log
# directory for the file-based handshake, and a name that scopes reaping.
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

# ── prepare project, state, and engine script inside crienv ─────────────────
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
docker exec "$CRIENV" mkdir -p /cri-smoke/project /cri-smoke/state
docker cp "$WORK/project/Pulumi.yaml" "$CRIENV:/cri-smoke/project/Pulumi.yaml"

# The engine runs this script inside the CRI container. Writing it as a file
# avoids the JSON-in-bash quoting nightmare of embedding shell in CRI args.
cat > "$WORK/engine-run.sh" <<'SCRIPT'
#!/bin/sh
set -e
pulumi login "$PULUMI_BACKEND_URL"
pulumi stack select --create "$STACK"
pulumi up --yes --skip-preview --stack "$STACK"
printf 'SMOKE greeting=<<%s>> hostname=<<%s>>\n' \
  "$(pulumi stack output greeting --stack "$STACK")" \
  "$(pulumi stack output hostname --stack "$STACK")"
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
    { "key": "PULUMI_POD_RUNTIME",     "value": "cri" },
    { "key": "PULUMI_POD_SANDBOX_ID",  "value": "$SB" },
    { "key": "PULUMI_POD_LOG_DIR",     "value": "$LOGDIR" },
    { "key": "PULUMI_POD_ID",          "value": "$POD_ID" },
    { "key": "PULUMI_POD_VOLUME_DIR",    "value": "$VOLDIR" },
    { "key": "PULUMI_POD_PROGRAM_IMAGE", "value": "$PROGRAM_IMAGE" },
    { "key": "PULUMI_BACKEND_URL",     "value": "file:///state" },
    { "key": "PULUMI_CONFIG_PASSPHRASE", "value": "smoke-test" },
    { "key": "STACK",                  "value": "$STACK" }
  ],
  "mounts": [
    { "host_path": "/run/containerd/containerd.sock", "container_path": "/run/containerd/containerd.sock" },
    { "host_path": "$LOGDIR", "container_path": "$LOGDIR" },
    { "host_path": "$VOLDIR", "container_path": "$VOLDIR" },
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
TIMEOUT=120
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
  echo "!! CRI pod-up smoke test FAIL"
  echo ""
  echo "This is expected for the first run — the point is to see WHAT broke."
  echo "Check the engine logs above for the first error."
  exit 1
fi

if grep -q 'SMOKE greeting=<<.*OCI runtime.*>>' "$WORK/engine.log"; then
  echo "    $(grep -o 'SMOKE greeting=.*' "$WORK/engine.log" | head -1)"
  echo "==> CRI pod-up smoke test PASS"
else
  echo "!! expected greeting output not found in logs"
  echo "!! CRI pod-up smoke test FAIL (engine exited 0 but output is unexpected)"
  exit 1
fi
