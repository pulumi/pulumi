#!/usr/bin/env bash
#
# CRI provider smoke test — and the ADDRESS-LAYER consume proof. The engine starts
# the random provider as a sibling CRI container, the program creates a RandomPet
# through it, and the test asserts the output. This proves the full provider boot
# path through the CRI pod manager: image pull, RunContainer for the provider,
# scrapeServingPort from CRI logs, NewProviderAttached over loopback, and the
# workspace volume mount shared between program and provider.
#
# What makes this the address-layer proof: `random` is an UNPINNED first-party
# provider, so the engine resolves it under the baked constant public source
# (oci.DefaultPublicRegistry = pulumi.registry.internal) — a stable, made-up
# hostname that is pure IDENTITY. There is no registry knob and no pre-loaded
# image. The provider image is PULLED: containerd maps the identity hostname to the
# proxy's real endpoint (http://10.88.0.1:5000) through a certs.d hosts.toml — the
# address layer — and the proxy synthesizes the image from the released binary. So
# identity (the ref's host) and location (the hosts.toml endpoint) are decoupled,
# exactly as production DNS/mirror config would do, with nothing rewritten.
#
# The `random` provider is stateless — it runs from its own image, not the
# program image — so it avoids CopyFromImage (the binary-injection path used by
# `command`). That path is still stubbed and is a separate test.
#
# Prerequisites: same as run-pod-cri.sh (docker, crienv, Go toolchain), and crienv
# must have outbound access to get.pulumi.com (the proxy synthesizes from it).
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
# The address layer under test. An unpinned provider resolves under the baked
# constant public source (a stable hostname = pure identity); containerd maps that
# identity to the proxy's real endpoint via a certs.d hosts.toml, so the engine
# pulls the synthesized image over the address layer with nothing pre-loaded.
PUBLIC_HOSTNAME="pulumi.registry.internal" # = oci.DefaultPublicRegistry
REG_HOST="10.88.0.1"                       # the cri0 gateway (reachable both-netns)
PUBLIC_ENDPOINT="http://$REG_HOST:5000"    # where the proxy public port actually lives
CERTS_D=/etc/containerd/certs.d
# The ref the engine computes for random, and the image containerd must pull.
PROVIDER_REF="$PUBLIC_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
POD_ID="cri-prov-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
STACK="dev"

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
  docker exec "$CRIENV" crictl rmi "$PROVIDER_REF" >/dev/null 2>&1 || true
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

# The provider image is NOT built or pre-loaded — that is the point. It is pulled
# through the address layer (the registry-proxy synthesizes it on demand).

# ── load engine + program images into crienv's k8s.io store ────────────────
for img in "$ENGINE_IMAGE" "$PROGRAM_IMAGE"; do
  echo "==> loading $img into crienv k8s.io store"
  docker save "$img" | docker exec -i "$CRIENV" ctr -n k8s.io images import -
done

# ── start the registry-proxy (public port) inside crienv ────────────────────
# The public port synthesizes first-party pulumi/pulumi-provider-* on demand from
# get.pulumi.com. It binds the cri0 gateway so containerd (crienv host netns) reaches
# it. The binary was cross-compiled by build_engine_image.
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
echo "   registry-proxy up on $PUBLIC_ENDPOINT"

# ── the address layer: map the identity hostname to the proxy endpoint ──────
# certs.d/<identity-host>/hosts.toml. The directory names the IDENTITY host the ref
# carries (pulumi.registry.internal); the [host."..."] entry is the real ADDRESS
# (10.88.0.1:5000, plain-http). containerd dials the endpoint and never DNS-resolves
# the identity — the decoupling the whole design rests on. Hot-reloads (config_path
# is /etc/containerd/certs.d).
echo "==> writing hosts.toml: $PUBLIC_HOSTNAME -> $PUBLIC_ENDPOINT"
docker exec "$CRIENV" sh -c "mkdir -p '$CERTS_D/$PUBLIC_HOSTNAME' && cat > '$CERTS_D/$PUBLIC_HOSTNAME/hosts.toml' <<TOML
server = \"https://$PUBLIC_HOSTNAME\"
[host.\"$PUBLIC_ENDPOINT\"]
  capabilities = [\"pull\", \"resolve\"]
TOML"

# Remove any image from a prior run so the pull must happen fresh.
docker exec "$CRIENV" crictl rmi "$PROVIDER_REF" >/dev/null 2>&1 || true

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

# The address-layer proof: random was PULLED (not pre-loaded) under the stable
# identity hostname, which containerd resolved to the proxy endpoint via hosts.toml.
if ! grep -q "oci: installed plugin $PROVIDER_REF by pulling its image" "$WORK/engine.log"; then
  echo "!! random was not pulled under $PROVIDER_REF — the address layer did not route the pull"
  grep -n "oci: .*plugin" "$WORK/engine.log" || true
  exit 1
fi
echo "    address layer: random pulled as $PROVIDER_REF (identity) via $PUBLIC_ENDPOINT (address)"

# Corroborate at the other end: the pull actually reached the proxy, which synthesized
# the image — so the hosts.toml endpoint mapping was exercised, not a lucky store hit.
docker exec "$CRIENV" cat /tmp/proxy.log >"$WORK/proxy.log" 2>&1 || true
if ! grep -q "synthesizing pulumi/pulumi-provider-$PROVIDER_PKG" "$WORK/proxy.log"; then
  echo "!! the proxy did not synthesize random — the pull did not traverse the address layer"
  cat "$WORK/proxy.log" || true
  exit 1
fi
echo "    proxy synthesized pulumi/pulumi-provider-$PROVIDER_PKG on demand — the endpoint was reached"

PET="$(sed -n 's/.*SMOKE petName=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET" ]; then
  echo "!! no petName output — the provider did not create the resource"
  exit 1
fi
echo "    petName = $PET"
echo "==> CRI provider smoke test PASS — provider consumed through the address layer"
