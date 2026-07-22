#!/usr/bin/env bash
#
# CRI MULTI-SOURCE consume proof — the true prize: a first-party AND a private
# package resolved to SEPARATE registries in ONE program. The program creates a
# resource through each of two explicit `random` providers:
#
#   pub  — UNPINNED. Resolves by convention under the constant public source
#          (oci.DefaultPublicRegistry = pulumi.registry.internal); the proxy's PUBLIC
#          port synthesizes it from the released binary.
#   priv — the SAME package (pulumi/pulumi-provider-random) PINNED to a private
#          source (oci://private.registry.internal/...); its ref names its own host,
#          so it resolves there verbatim, served by the proxy's PRIVATE port.
#
# Same publisher, same name, DIFFERENT source — the exact case one registry knob
# could never express (one knob = one host). It works now because resolution is
# source-preserving (pins verbatim, unpinned under the constant) and because the
# ADDRESS LAYER decouples identity from location: each identity host is a stable,
# made-up hostname that is pure identity, mapped to the proxy's real endpoint by a
# containerd certs.d hosts.toml. containerd dials the endpoint and never DNS-resolves
# the hostname — nothing is rewritten. Explicit providers are keyed by URN, not
# package name, so each carries its own pin into the descriptor and the container
# host resolves each to its own image (a first-party synth vs a private pushed copy).
#
# Nothing is pre-loaded: both provider images are PULLED, each from its own port. The
# private copy is staged by pulling the public synth and pushing it to the private
# port (setup only; the engine's pulls are fresh). random is stateless (runs from its
# own image), so this avoids the CopyFromImage binary-injection path.
#
# Prerequisites: same as run-pod-cri.sh (docker, crienv, Go toolchain), and crienv
# must have outbound access to get.pulumi.com (the public port synthesizes from it).
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
# The two sources under test. Each identity host is a stable, made-up hostname that
# is pure identity; a certs.d hosts.toml maps it to the proxy's real endpoint (both
# on the cri0 gateway, different ports). The engine pulls each provider image from
# its own port with nothing pre-loaded.
REG_HOST="10.88.0.1" # the cri0 gateway (reachable both-netns)
CERTS_D=/etc/containerd/certs.d
PUBLIC_HOSTNAME="pulumi.registry.internal"   # = oci.DefaultPublicRegistry (unpinned convention)
PRIVATE_HOSTNAME="private.registry.internal" # the private source a pin names
PUBLIC_ENDPOINT="http://$REG_HOST:5000"      # proxy PUBLIC port (synthesis, read-only)
PRIVATE_ENDPOINT="http://$REG_HOST:5001"     # proxy PRIVATE port (plain read-write registry)
# The refs the engine computes and containerd must pull — one per source.
PUBLIC_REF="$PUBLIC_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
PRIVATE_REF="$PRIVATE_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
# The direct endpoint refs used to stage the private copy (setup only). Staging runs
# BEFORE any sandbox exists, so it uses loopback (always present) rather than the cri0
# gateway 10.88.0.1 (which only appears once a sandbox brings up the bridge); the proxy
# binds 0.0.0.0 so both reach the same registry storage.
PUBLIC_DIRECT="127.0.0.1:5000/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
PRIVATE_DIRECT="127.0.0.1:5001/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
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
  docker exec "$CRIENV" rm -rf "$CERTS_D/$PUBLIC_HOSTNAME" "$CERTS_D/$PRIVATE_HOSTNAME" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmi "$PUBLIC_REF" "$PRIVATE_REF" "$PUBLIC_DIRECT" "$PRIVATE_DIRECT" >/dev/null 2>&1 || true
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

# ── start the registry-proxy (BOTH ports) inside crienv ─────────────────────
# Public :5000 synthesizes first-party pulumi/pulumi-provider-* on demand from
# get.pulumi.com; private :5001 is a plain read-write registry (the private source).
# Both bind the cri0 gateway so containerd (crienv host netns) reaches them. The
# binary was cross-compiled by build_engine_image.
echo "==> starting registry-proxy (public :5000 + private :5001) in crienv"
docker cp "$WORK/cli/registry-proxy-linux" "$CRIENV:/usr/local/bin/registry-proxy"
docker exec "$CRIENV" pkill -f 'registry-proxy|plainregistry' 2>/dev/null || true
sleep 1
docker exec -d "$CRIENV" sh -c 'PROXY_ADDR=:5000 PROXY_PRIVATE_ADDR=:5001 /usr/local/bin/registry-proxy >/tmp/proxy.log 2>&1'
for _ in $(seq 1 15); do
  docker exec "$CRIENV" sh -c 'curl -sf http://127.0.0.1:5000/v2/ >/dev/null 2>&1 && curl -sf http://127.0.0.1:5001/v2/ >/dev/null 2>&1' && break; sleep 1
done
docker exec "$CRIENV" sh -c 'curl -sf http://127.0.0.1:5000/v2/ >/dev/null 2>&1 && curl -sf http://127.0.0.1:5001/v2/ >/dev/null 2>&1' || {
  echo "!! registry-proxy did not come up on :5000 + :5001"; docker exec "$CRIENV" cat /tmp/proxy.log; exit 1; }
echo "   registry-proxy up: public $PUBLIC_ENDPOINT, private $PRIVATE_ENDPOINT"

# ── stage the private copy of random (setup only) ───────────────────────────
# The private port is a bare registry — it synthesizes nothing, so the private
# provider image must be pushed there first. Pull the public synth by its DIRECT
# endpoint ref (distinct from the identity ref the engine will use), retag to the
# private endpoint, push, then remove BOTH staging refs so the engine's two pulls
# are fresh (the pushed blobs live in the private registry, independent of the store).
echo "==> staging the private copy: pull public synth -> push to private port"
docker exec "$CRIENV" ctr -n k8s.io images pull --plain-http "$PUBLIC_DIRECT" >/dev/null
docker exec "$CRIENV" ctr -n k8s.io images tag "$PUBLIC_DIRECT" "$PRIVATE_DIRECT" >/dev/null
docker exec "$CRIENV" ctr -n k8s.io images push --plain-http "$PRIVATE_DIRECT" >/dev/null
docker exec "$CRIENV" ctr -n k8s.io images rm "$PUBLIC_DIRECT" "$PRIVATE_DIRECT" >/dev/null
echo "   private port now serves pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

# ── the address layer: map each identity hostname to its proxy endpoint ──────
# certs.d/<identity-host>/hosts.toml. The directory names the IDENTITY host the ref
# carries; the [host."..."] entry is the real ADDRESS (plain-http). containerd dials
# the endpoint and never DNS-resolves the identity — the decoupling the design rests
# on. Two entries = two sources. Hot-reloads (config_path is /etc/containerd/certs.d).
write_hosts_toml() { # identity-host endpoint
  docker exec "$CRIENV" sh -c "mkdir -p '$CERTS_D/$1' && cat > '$CERTS_D/$1/hosts.toml' <<TOML
server = \"https://$1\"
[host.\"$2\"]
  capabilities = [\"pull\", \"resolve\"]
TOML"
}
echo "==> writing hosts.toml: $PUBLIC_HOSTNAME -> $PUBLIC_ENDPOINT, $PRIVATE_HOSTNAME -> $PRIVATE_ENDPOINT"
write_hosts_toml "$PUBLIC_HOSTNAME" "$PUBLIC_ENDPOINT"
write_hosts_toml "$PRIVATE_HOSTNAME" "$PRIVATE_ENDPOINT"

# Remove any image from a prior run so the pulls must happen fresh.
docker exec "$CRIENV" crictl rmi "$PUBLIC_REF" "$PRIVATE_REF" >/dev/null 2>&1 || true

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
printf 'SMOKE petPub=<<%s>>\n' "$(pulumi stack output petPub --stack "$STACK")"
printf 'SMOKE petPriv=<<%s>>\n' "$(pulumi stack output petPriv --stack "$STACK")"
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

# Two provider containers must have started — one per source. If the two same-name
# providers had collapsed to one image, there would be a single container and the
# demo would prove nothing while still printing two pets.
CONTAINERS="$(grep -c 'oci: provider random running as container' "$WORK/engine.log" || true)"
if [ "$CONTAINERS" -lt 2 ]; then
  echo "!! expected two provider containers (one per source), saw $CONTAINERS — the sources collapsed"
  grep -n 'running as container' "$WORK/engine.log" || true
  exit 1
fi

# The multi-source proof: BOTH images were pulled, each under its OWN identity host.
# Same pulumi/pulumi-provider-random, two different hosts — resolved independently.
for ref in "$PUBLIC_REF" "$PRIVATE_REF"; do
  if ! grep -q "oci: installed plugin $ref by pulling its image" "$WORK/engine.log"; then
    echo "!! $ref was not pulled — a source did not resolve independently"
    grep -n 'oci: .*plugin' "$WORK/engine.log" || true
    exit 1
  fi
done
echo "    first-party pulled as  $PUBLIC_REF  (-> $PUBLIC_ENDPOINT)"
echo "    private    pulled as  $PRIVATE_REF (-> $PRIVATE_ENDPOINT)"
echo "    same pulumi/pulumi-provider-random, TWO sources, resolved independently in one program"

# Corroborate at the proxy. The PUBLIC port logs its synthesis, so a synth line proves
# :5000 was reached. The PRIVATE ref can ONLY route (via its hosts.toml) to :5001, and
# its image exists nowhere else (staging refs were removed from the store; the public
# port never serves the private host), so its successful pull above proves :5001 was
# hit — the two hosts.toml entries routed the two pulls to two ports.
docker exec "$CRIENV" cat /tmp/proxy.log >"$WORK/proxy.log" 2>&1 || true
if ! grep -q "synthesizing pulumi/pulumi-provider-$PROVIDER_PKG" "$WORK/proxy.log"; then
  echo "!! the public port did not synthesize random — the public pull did not traverse the address layer"
  cat "$WORK/proxy.log" || true
  exit 1
fi
echo "    proxy public port synthesized on demand — the public endpoint was reached"

PET_PUB="$(sed -n 's/.*SMOKE petPub=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
PET_PRIV="$(sed -n 's/.*SMOKE petPriv=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET_PUB" ] || [ -z "$PET_PRIV" ]; then
  echo "!! missing pet output — pub='$PET_PUB' priv='$PET_PRIV' (a provider did not create its resource)"
  exit 1
fi
echo "    petPub (first-party) = $PET_PUB"
echo "    petPriv (private)    = $PET_PRIV"
echo "==> CRI MULTI-SOURCE smoke test PASS — first-party and private packages resolved to separate registries in one program"
