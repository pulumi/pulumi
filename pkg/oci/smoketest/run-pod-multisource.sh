#!/usr/bin/env bash
#
# DOCKER multi-source consume proof — the docker twin of run-pod-cri-provider.sh.
# One program, two explicit `random` providers, SAME package
# (pulumi/pulumi-provider-random) resolved to SEPARATE registries:
#
#   pub  — UNPINNED. Resolves by convention under the constant public source
#          (pulumi.registry.internal); the proxy's PUBLIC port synthesizes it.
#   priv — the SAME package PINNED to a private source
#          (oci://private.registry.internal/...); the proxy's PRIVATE port serves it.
#
# It runs the IDENTICAL program and pins as the CRI proof — the only difference is the
# ADDRESS mechanism. Docker pulls a ref itself via the host daemon (it only auto-trusts
# loopback as insecure and reads no injectable per-registry endpoint config), so it
# cannot reach a made-up identity host the way containerd's hosts.toml does. The docker
# analog is a pull-time remap in the pod manager, configured by PULUMI_POD_REGISTRY_ENDPOINTS
# (identity-host=address-endpoint): the manager pulls from the endpoint and tags the
# result back to the identity ref, so the image runs under its real name. Identity is
# never rewritten (== CRI == production); only the pull address is redirected.
#
# Nothing is pre-loaded: both provider images are PULLED, each from its own port. The
# private copy is staged by pulling the public synth and pushing it to the private port
# (setup only; the engine's pulls are fresh).
#
# Each provider is its own container on the pod network, attached at its own address
# on the forwarder shim's well-known port. This is the case where the shim is
# load-bearing — two providers of the SAME package attach at the SAME port :7777 and
# can only be told apart by their addresses, which a shared netns could not express.
#
# Usage: run-pod-multisource.sh
# Requires a running Docker daemon (with outbound access to get.pulumi.com, which the
# public port synthesizes from) and the repo Go toolchain.
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # cross-compile CLI + proxy, build engine image
PROJECT_DIR="$SMOKE_DIR/project-random"
PROGRAM_DIR="$SMOKE_DIR/program-random"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

PROXY_SHIM_BIN="/shim" # synthesized images boot through the shim
MODE_LABEL="each provider its own container, attached at its address on the shim port :7777"

POD_ID="msrc-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-random:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

# The two sources. Each identity host is a stable, made-up hostname (== the CRI proof);
# the endpoint map redirects the PULL to the proxy's published localhost port. The host
# daemon does the pull, so the endpoints are host-reachable (localhost:PORT, auto-insecure).
PROXY_NAME="proto-multisource-proxy-$$"
REG_PUB_PORT=5074
REG_PRIV_PORT=5075
PUBLIC_HOSTNAME="pulumi.registry.internal"   # = oci.DefaultPublicRegistry (unpinned convention)
PRIVATE_HOSTNAME="private.registry.internal" # the private source a pin names
PUBLIC_ENDPOINT="localhost:$REG_PUB_PORT"    # proxy PUBLIC port (synthesis), host-published
PRIVATE_ENDPOINT="localhost:$REG_PRIV_PORT"  # proxy PRIVATE port (read-write), host-published
REGISTRY_ENDPOINTS="$PUBLIC_HOSTNAME=$PUBLIC_ENDPOINT,$PRIVATE_HOSTNAME=$PRIVATE_ENDPOINT"
# The identity refs the engine computes and runs the providers under.
PUBLIC_REF="$PUBLIC_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
PRIVATE_REF="$PRIVATE_HOSTNAME/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
# The published-endpoint refs used to stage the private copy (setup only).
PUBLIC_STAGE="$PUBLIC_ENDPOINT/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
PRIVATE_STAGE="$PRIVATE_ENDPOINT/pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  # Pod-scoped volumes (the workspace, and any plugin-injection volume) outlive their
  # containers. The engine reaps them on a clean Close(), so this only matters when a run
  # dies first — but that is exactly when it matters: each aborted run otherwise strands a
  # workspace volume, and enough of them fill the daemon's disk.
  local vols
  vols="$(docker volume ls -q --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$vols" ] && docker volume rm $vols >/dev/null 2>&1 || true
  # Identity refs + staging refs this run owns.
  docker image rm -f "$PUBLIC_REF" "$PRIVATE_REF" "$PUBLIC_STAGE" "$PRIVATE_STAGE" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run multi-source test"; exit 1
fi

build_engine_image

echo "==> cross-compiling program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

# ── start the registry-proxy (BOTH ports) as a host container ───────────────
# Public :5000 synthesizes first-party pulumi/pulumi-provider-* from get.pulumi.com;
# private :5001 is a plain read-write registry. Both are published to the host so the
# engine's host-daemon pulls reach them at localhost:PORT (auto-insecure).
# In address mode PULUMI_POD_SHIM_BIN is set, so every image the public port synthesizes
# gets the shim embedded and its entrypoint rewritten to boot through it. This must be true
# BEFORE the staging pull below: the private copy is a push of the public synth, so a proxy
# started shim-off would strand the private provider shimless while the public one works.
echo "==> starting registry-proxy (public :$REG_PUB_PORT + private :$REG_PRIV_PORT)"
docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
docker run -d --name "$PROXY_NAME" -p "$REG_PUB_PORT:5000" -p "$REG_PRIV_PORT:5001" \
  -e PROXY_TARGET_ARCH="$GOARCH" -e PROXY_PRIVATE_ADDR=":5001" \
  -e PULUMI_POD_SHIM_BIN="$PROXY_SHIM_BIN" \
  -v "$WORK/cli/registry-proxy-linux":/registry-proxy:ro \
  -v "$WORK/cli/pulumi-pod-shim-linux":/shim:ro \
  alpine sh -c 'apk add --no-cache ca-certificates >/dev/null 2>&1 && exec /registry-proxy' >/dev/null
for _ in $(seq 1 30); do
  curl -sf "http://$PUBLIC_ENDPOINT/v2/" >/dev/null 2>&1 && curl -sf "http://$PRIVATE_ENDPOINT/v2/" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -sf "http://$PUBLIC_ENDPOINT/v2/" >/dev/null 2>&1 && curl -sf "http://$PRIVATE_ENDPOINT/v2/" >/dev/null 2>&1 || {
  echo "!! registry-proxy did not come up on $PUBLIC_ENDPOINT + $PRIVATE_ENDPOINT"; docker logs "$PROXY_NAME" 2>&1 | tail -20; exit 1; }
echo "   registry-proxy up: public $PUBLIC_ENDPOINT, private $PRIVATE_ENDPOINT"

# ── stage the private copy of random (setup only) ───────────────────────────
# The private port synthesizes nothing, so the private provider image must be pushed
# there first: pull the public synth by its endpoint ref, retag to the private endpoint,
# push, then remove BOTH staging tags so the engine's two pulls are fresh (the pushed
# blobs live in the private registry, independent of the local store).
echo "==> staging the private copy: pull public synth -> push to private port"
docker pull -q "$PUBLIC_STAGE" >/dev/null
docker tag "$PUBLIC_STAGE" "$PRIVATE_STAGE"
docker push -q "$PRIVATE_STAGE" >/dev/null
docker image rm -f "$PUBLIC_STAGE" "$PRIVATE_STAGE" >/dev/null 2>&1 || true
# Also clear any identity-ref tags from a prior run so the pulls must happen fresh.
docker image rm -f "$PUBLIC_REF" "$PRIVATE_REF" >/dev/null 2>&1 || true
echo "   private port now serves pulumi/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME (endpoint map remaps each pull to its port)"
echo "    provider attach mode: $MODE_LABEL"
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
  -e PULUMI_POD_REGISTRY_ENDPOINTS="$REGISTRY_ENDPOINTS" \
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
    printf "SMOKE petPub=<<%s>>\n" "$(pulumi stack output petPub --stack "$STACK")"
    printf "SMOKE petPriv=<<%s>>\n" "$(pulumi stack output petPriv --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

# ── assertions ──────────────────────────────────────────────────────────────
echo "==> checking results"

# Two provider containers must have started — one per source, not a dedup'd single.
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
echo "    first-party pulled as  $PUBLIC_REF  (address-remapped to $PUBLIC_ENDPOINT)"
echo "    private    pulled as  $PRIVATE_REF (address-remapped to $PRIVATE_ENDPOINT)"
echo "    same pulumi/pulumi-provider-random, TWO sources, resolved independently in one program"

# Corroborate at the proxy: the public port logs its synthesis, so a synth line proves
# the public endpoint was reached (a fresh pull, not a store hit). The private ref's
# image exists only on the private port (staging tags removed), so its pull proves the
# private endpoint was reached.
docker logs "$PROXY_NAME" >"$WORK/proxy.log" 2>&1 || true
if ! grep -q "synthesizing pulumi/pulumi-provider-$PROVIDER_PKG" "$WORK/proxy.log"; then
  echo "!! the public port did not synthesize random — the public pull did not traverse the address layer"
  tail -20 "$WORK/proxy.log" || true
  exit 1
fi
echo "    proxy public port synthesized on demand — the public endpoint was reached"

echo "==> asserting each provider was attached at its OWN address"
if grep -q 'attaching at 127.0.0.1' "$WORK/engine.log"; then
  echo "!! a provider was attached over 127.0.0.1 — providers are not being dialed on the pod network"
  grep -n 'attaching at' "$WORK/engine.log" || true
  exit 1
fi
# The claim a shared netns could not make: two attaches at the SAME well-known port,
# told apart only by address. Distinct hosts also rule out one container being
# attached twice, which a bare count of ":7777" lines would not.
ATTACH_HOSTS="$(sed -n 's/.*attaching at \([^ :]*\):7777.*/\1/p' "$WORK/engine.log" | sort -u)"
DISTINCT="$(printf '%s\n' "$ATTACH_HOSTS" | grep -c . || true)"
if [ "$DISTINCT" -lt 2 ]; then
  echo "!! expected two DISTINCT provider hosts at the shim port :7777, saw $DISTINCT"
  grep -n 'attaching at' "$WORK/engine.log" || true
  exit 1
fi
echo "    attached at: $(printf '%s\n' "$ATTACH_HOSTS" | tr '\n' ' ')"
echo "    same package, two addresses, one shared port :7777 — the collision a shared netns could not express"

PET_PUB="$(sed -n 's/.*SMOKE petPub=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
PET_PRIV="$(sed -n 's/.*SMOKE petPriv=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
if [ -z "$PET_PUB" ] || [ -z "$PET_PRIV" ]; then
  echo "!! missing pet output — pub='$PET_PUB' priv='$PET_PRIV' (a provider did not create its resource)"
  exit 1
fi
echo "    petPub (first-party) = $PET_PUB"
echo "    petPriv (private)    = $PET_PRIV"
echo "==> DOCKER MULTI-SOURCE smoke test PASS ($MODE_LABEL)"
echo "    first-party and private packages resolved to separate registries in one program"
