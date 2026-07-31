#!/usr/bin/env bash
#
# CRI twin of run-pod-transform.sh — the SAME program and the SAME observable, on the other
# runtime, to settle whether the transform failure is a Pulumi-level gap or a docker-topology
# one.
#
# The claim under test: providers netns-join on every runtime, but the PROGRAM does not. On
# docker/nerdctl it is a sibling on the pod bridge with its own netns, so the engine dialing
# the program's advertised 127.0.0.1 callback address reaches itself and the transform fails.
# On CRI every pod member shares the one sandbox netns ("network": 0 below is CRI's POD
# namespace mode), so loopback IS shared — the engine even advertises itself as 127.0.0.1 —
# and the callback should resolve.
#
# If this passes while run-pod-transform.sh fails, transforms are not broken in "pod mode";
# they are broken wherever the program has its own netns, which is a topology property and
# exactly what an addressable bind contract normalizes away.
#
# The provider setup is inherited from run-pod-cri-provider.sh but single-source: the program
# uses the DEFAULT (unpinned) random provider, which resolves by convention under
# oci.DefaultPublicRegistry and is synthesized on demand by the proxy's public port. Nothing
# is pre-loaded; the image is pulled through the address layer.
#
# Prerequisites: docker, a running crienv container, the Go toolchain, and crienv outbound
# access to get.pulumi.com.
# FROZEN (2026-07-30): CRI served as the second-container-runtime pressure test and is
# retired from maintenance. The engine now dials plugins by pod-network IP; CRI
# containers share one sandbox IP, so these tests no longer hold against current
# engines (a thaw means sandbox-per-plugin). ac593679 is the last commit with the
# netns machinery these tests were written against.
echo "!! CRI smoke tests are FROZEN — the engine now dials plugins by pod-network IP," >&2
echo "   which CRI's shared-sandbox topology cannot serve. Check out ac593679 or earlier." >&2
exit 3

set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-transform"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

CRIENV=crienv
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

REG_HOST="10.88.0.1" # the cri0 gateway (reachable from both netns)
CERTS_D=/etc/containerd/certs.d
PUBLIC_HOSTNAME="pulumi.registry.internal" # = oci.DefaultPublicRegistry (unpinned convention)
PUBLIC_ENDPOINT="http://$REG_HOST:5000"    # proxy PUBLIC port (synthesis, read-only)
PUBLIC_REF="$PUBLIC_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

POD_ID="cri-xform-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
STACK="dev"
# Pod-scoped project/state inside crienv. The other CRI scripts share /cri-smoke/state and
# all use stack "dev", so a run inherits whatever the last one left — a prior multi-source
# run's state names an oci:// pin to a private registry this test never starts, and `up`
# fails trying to reconcile it. Scope the state to this run so the test measures itself.
SMOKE_ROOT="/cri-smoke/$POD_ID"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  echo "== cleanup =="
  if [ -n "${SB:-}" ]; then
    docker exec "$CRIENV" crictl stopp "$SB" >/dev/null 2>&1 || true
    docker exec "$CRIENV" crictl rmp -f "$SB" >/dev/null 2>&1 || true
    echo "   reaped sandbox $SB"
  fi
  docker exec "$CRIENV" pkill -f 'registry-proxy' >/dev/null 2>&1 || true
  docker exec "$CRIENV" rm -rf "$CERTS_D/$PUBLIC_HOSTNAME" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmi "$PUBLIC_REF" >/dev/null 2>&1 || true
  docker exec "$CRIENV" rm -rf "$SMOKE_ROOT" >/dev/null 2>&1 || true
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

build_engine_image

echo "==> cross-compiling transform program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load -q \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR" >/dev/null

for img in "$ENGINE_IMAGE" "$PROGRAM_IMAGE"; do
  echo "==> loading $img into crienv k8s.io store"
  docker save "$img" | docker exec -i "$CRIENV" ctr -n k8s.io images import - >/dev/null
done

echo "==> starting registry-proxy (public :5000) in crienv"
docker cp "$WORK/cli/registry-proxy-linux" "$CRIENV:/usr/local/bin/registry-proxy"
docker exec "$CRIENV" pkill -f 'registry-proxy|plainregistry' 2>/dev/null || true
sleep 1
docker exec -d "$CRIENV" sh -c 'PROXY_ADDR=:5000 /usr/local/bin/registry-proxy >/tmp/proxy.log 2>&1'
for _ in $(seq 1 15); do
  docker exec "$CRIENV" sh -c 'curl -sf http://127.0.0.1:5000/v2/ >/dev/null 2>&1' && break; sleep 1
done
docker exec "$CRIENV" sh -c 'curl -sf http://127.0.0.1:5000/v2/ >/dev/null 2>&1' || {
  echo "!! registry-proxy did not come up on :5000"; docker exec "$CRIENV" cat /tmp/proxy.log; exit 1; }
echo "   registry-proxy up: public $PUBLIC_ENDPOINT"

write_hosts_toml() { # identity-host endpoint
  docker exec "$CRIENV" sh -c "mkdir -p '$CERTS_D/$1' && cat > '$CERTS_D/$1/hosts.toml' <<TOML
server = \"https://$1\"
[host.\"$2\"]
  capabilities = [\"pull\", \"resolve\"]
TOML"
}
echo "==> writing hosts.toml: $PUBLIC_HOSTNAME -> $PUBLIC_ENDPOINT"
write_hosts_toml "$PUBLIC_HOSTNAME" "$PUBLIC_ENDPOINT"
docker exec "$CRIENV" crictl rmi "$PUBLIC_REF" >/dev/null 2>&1 || true

for p in $(docker exec "$CRIENV" crictl pods --name "$POD_ID" -q 2>/dev/null); do
  docker exec "$CRIENV" crictl stopp "$p" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmp -f "$p" >/dev/null 2>&1 || true
done

# ── create the pod sandbox ─────────────────────────────────────────────────
# "network": 0 is CRI's POD namespace mode: every container in this sandbox shares one
# network namespace. That shared loopback is the whole variable under test.
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

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"
docker exec "$CRIENV" mkdir -p "$SMOKE_ROOT/project" "$SMOKE_ROOT/state"
docker cp "$WORK/project/Pulumi.yaml" "$CRIENV:$SMOKE_ROOT/project/Pulumi.yaml"

cat > "$WORK/engine-run.sh" <<'SCRIPT'
#!/bin/sh
set -e
pulumi login "$PULUMI_BACKEND_URL"
pulumi stack select --create "$STACK"
pulumi up --yes --skip-preview --stack "$STACK"
printf 'SMOKE petName=<<%s>>\n' "$(pulumi stack output petName --stack "$STACK")"
SCRIPT
docker cp "$WORK/engine-run.sh" "$CRIENV:$SMOKE_ROOT/engine-run.sh"
docker exec "$CRIENV" chmod +x "$SMOKE_ROOT/engine-run.sh"

echo "==> creating engine container in sandbox $SB"
cat > "$WORK/engine-container.json" <<JSON
{
  "metadata": { "name": "engine", "attempt": 0 },
  "image": { "image": "$ENGINE_IMAGE" },
  "command": ["/bin/sh"],
  "args": ["$SMOKE_ROOT/engine-run.sh"],
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
    { "host_path": "$SMOKE_ROOT/project", "container_path": "/project" },
    { "host_path": "$SMOKE_ROOT/state",   "container_path": "/state" },
    { "host_path": "$SMOKE_ROOT",         "container_path": "$SMOKE_ROOT" }
  ],
  "log_path": "engine_0.log"
}
JSON
docker cp "$WORK/engine-container.json" "$CRIENV:/tmp/engine-container.json"

ENGINE_ID="$(docker exec "$CRIENV" crictl create "$SB" /tmp/engine-container.json /tmp/sandbox.json)"
echo "   engine container: $ENGINE_ID"
docker exec "$CRIENV" crictl start "$ENGINE_ID"

echo "==> waiting for engine container to exit..."
TIMEOUT=240
ELAPSED=0
while true; do
  STATE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.state}}' "$ENGINE_ID" 2>/dev/null || echo "unknown")"
  [ "$STATE" = "CONTAINER_EXITED" ] && break
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "!! engine container did not exit within ${TIMEOUT}s"
    docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 || true
    exit 1
  fi
  sleep 2
  ELAPSED=$((ELAPSED + 2))
done

EXIT_CODE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.exitCode}}' "$ENGINE_ID" 2>/dev/null || echo "-1")"
echo "==> engine exited with code $EXIT_CODE after ~${ELAPSED}s"

echo "== engine logs =="
docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 | tee "$WORK/engine.log"

# ── topology evidence ──────────────────────────────────────────────────────
# The docker twin records the program container's NetworkMode. The CRI analogue is that
# every container in the sandbox shares the sandbox's netns by construction — so record
# the sandbox's network namespace path and the engine's, which must be the same one.
# The sandbox names its netns by CNI path (/var/run/netns/cni-...) while a container names it
# by proc handle (/proc/<pid>/ns/net). Those strings are never equal even when they denote the
# SAME namespace, so compare the inode they resolve to — that is the identity of a namespace.
#
# This samples after the engine exits, so the proc handle can in principle be gone by now; when
# either inode fails to resolve the block says so and defers to the transform result, which is
# the functional proof either way. Do not read a "could not resolve" as a topology signal.
echo "==> topology evidence"
SB_NETNS="$(docker exec "$CRIENV" crictl inspectp --output go-template \
  --template '{{range .info.runtimeSpec.linux.namespaces}}{{if eq .type "network"}}{{.path}}{{end}}{{end}}' "$SB" 2>/dev/null || true)"
ENGINE_NETNS="$(docker exec "$CRIENV" crictl inspect --output go-template \
  --template '{{range .info.runtimeSpec.linux.namespaces}}{{if eq .type "network"}}{{.path}}{{end}}{{end}}' "$ENGINE_ID" 2>/dev/null || true)"
SB_INO="$(docker exec "$CRIENV" stat -L -c %i "$SB_NETNS" 2>/dev/null || true)"
ENGINE_INO="$(docker exec "$CRIENV" stat -L -c %i "$ENGINE_NETNS" 2>/dev/null || true)"
echo "    sandbox netns = ${SB_NETNS:-<none>} (inode ${SB_INO:-?})"
echo "    engine  netns = ${ENGINE_NETNS:-<none>} (inode ${ENGINE_INO:-?})"
if [ -n "$SB_INO" ] && [ "$SB_INO" = "$ENGINE_INO" ]; then
  echo "    -> SAME namespace: the engine is IN the sandbox netns, so its loopback is the pod's"
elif [ -n "$SB_INO" ] && [ -n "$ENGINE_INO" ]; then
  echo "    -> DIFFERENT namespaces — the shared-sandbox-netns premise does not hold here"
else
  echo "    (could not resolve both netns inodes; the transform result below is the functional proof)"
fi

# ── assertions ─────────────────────────────────────────────────────────────
echo "==> checking the transform actually applied"
PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
case "$PET" in
  transformed-*)
    echo "    petName = $PET (carries the transform's prefix)"
    echo "==> CRI TRANSFORM smoke test PASS — the engine dialed back into the program over the shared sandbox loopback"
    echo "    Contrast run-pod-transform.sh (docker), where the same program fails to be dialed back."
    ;;
  "")
    echo "!! no petName output — the update did not complete (engine exit $EXIT_CODE)"
    echo "   If this is a callback dial failure, then the shared-sandbox-netns reasoning is WRONG"
    echo "   and transforms are broken on CRI too — a much bigger finding than the docker case."
    grep -iE "callback|transform|connection refused|Unavailable|127\.0\.0\.1" "$WORK/engine.log" | tail -8 || true
    exit 1
    ;;
  *)
    echo "!! petName = $PET — created WITHOUT the transform's prefix, so the transform silently"
    echo "   did not run. A skipped transform is worse than a failed one: intent dropped, no error."
    exit 1
    ;;
esac
