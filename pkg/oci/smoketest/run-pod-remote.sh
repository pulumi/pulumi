#!/usr/bin/env bash
#
# REMOTE-execution smoke test — the whole pod runs on a *different* Docker daemon,
# reached only over the network, with the host contributing nothing but a docker
# client. This is the "one more thing": the same pod that runs locally runs on a
# remote daemon with ZERO product-code changes — the only deltas are where docker
# points and how the source gets there.
#
# Why this works at all (the load-bearing insight): the engine orchestrates the pod
# through the docker socket it has *mounted*, never via DOCKER_HOST. So wherever the
# engine container runs, the program/provider/builder containers it starts run on
# that SAME daemon — level-2 (engine -> children) follows level-1 (host -> engine)
# for free. "Remote execution" is therefore just "start the engine on a remote
# daemon"; everything downstream co-locates there automatically.
#
# We stand up a `docker:dind` daemon as a faithful stand-in for "remote": it has its
# OWN filesystem and image store and is reached over TCP, so a host bind mount of a
# host path would resolve inside dind (empty) — exactly the constraint a real remote
# executor imposes. If anything in the pod secretly depended on the host's disk, this
# test would fail.
#
# The deltas from the local template path (run-pod-template-nodejs.sh), and ONLY these:
#   1. point every pod docker command at the remote daemon (-H $REMOTE)
#   2. ship the engine image to the remote store (docker save | docker -H $REMOTE load).
#      The PROGRAM image is *built on the remote* by `up`, so it needs no transport; the
#      program has no external provider, so there is no provider image / registry either.
#   3. the workspace is the only thing the host must hand over. A host bind mount cannot
#      reach a remote daemon, so we seed a remote NAMED VOLUME with the project source and
#      mount that instead. Because the build container reads source via `--volumes-from`
#      the engine (volume-agnostic), this one move unlocks BOTH the remote build AND the
#      remote run — no separate "pre-build and push" path is needed.
#   4. the file backend lives inside that workspace volume, so state stays remote too —
#      no network backend required for the proof.
#
# So the only genuinely new mechanism is host-source -> remote-volume. Everything else is
# the existing pod, unchanged. (Once proven here, that seam folds into pkg/oci as a
# PodManager.CopyToVolume, symmetric with the existing CopyFromImage.)
#
# Usage: run-pod-remote.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
TEMPLATE_DIR="$SMOKE_DIR/templates/oci-nodejs"
PKG_DIR="$SMOKE_DIR/../.."

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point the
# engine-image build at a local builder. The dind container itself runs on the LOCAL
# daemon (it IS the remote); only the *pod* commands target $REMOTE.
BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
WSVOL="pulumi-pod-$POD_ID-workspace"
DIND_NAME="pulumi-pod-$POD_ID-remote"
STACK="dev"
PROJECT_NAME="oci-remote-smoke"
EXPECTED="hello from $PROJECT_NAME, an OCI nodejs program"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/stage"

REMOTE="" # set once dind is up

cleanup() {
  # Tearing down the dind container takes the entire remote pod with it (its network,
  # the workspace volume, and every program/provider/builder container live inside
  # dind's own daemon). Then drop the local scratch dir.
  docker rm -f "$DIND_NAME" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run remote-execution test"
  exit 1
fi

build_engine_image

# ---------------------------------------------------------------------------
# Stand up the "remote" daemon: a privileged docker:dind with TLS disabled so it
# serves plain TCP on 2375. Publish that to an ephemeral host port and read it back,
# so concurrent runs don't collide on a fixed port.
# ---------------------------------------------------------------------------
echo "==> starting the remote daemon (docker:dind) as a stand-in for a remote executor"
docker run -d --privileged --name "$DIND_NAME" \
  -e DOCKER_TLS_CERTDIR="" \
  -p 127.0.0.1::2375 \
  docker:dind --tls=false >/dev/null
DIND_PORT="$(docker port "$DIND_NAME" 2375/tcp | head -1 | sed 's/.*://')"
REMOTE="tcp://127.0.0.1:$DIND_PORT"
echo "    remote daemon reachable at $REMOTE"

echo "==> waiting for the remote daemon to accept connections"
for _ in $(seq 1 60); do
  docker -H "$REMOTE" info >/dev/null 2>&1 && break
  sleep 0.5
done
if ! docker -H "$REMOTE" info >/dev/null 2>&1; then
  echo "!! remote daemon did not come up at $REMOTE"
  docker logs "$DIND_NAME" 2>&1 | tail -20
  exit 1
fi

# ---------------------------------------------------------------------------
# Ship the engine image to the remote store. (The program image is built remotely by
# `up`; provider-less program => no provider image; so this is the only image to move.)
# ---------------------------------------------------------------------------
echo "==> shipping the engine image $ENGINE_IMAGE to the remote store (save | load)"
docker save "$ENGINE_IMAGE" | docker -H "$REMOTE" load >/dev/null

# ---------------------------------------------------------------------------
# Stage the project locally (substitute the template placeholders, ship the sdks/ dir
# the Dockerfile COPYs), then seed it into a remote NAMED VOLUME. This is the one new
# mechanism: a host bind can't reach a remote daemon, so the source travels as a tar
# streamed over the connection into a helper container that extracts it into the volume.
# ---------------------------------------------------------------------------
echo "==> staging the project and seeding it into the remote workspace volume $WSVOL"
cp -R "$TEMPLATE_DIR/." "$WORK/stage/"
# Concrete project (skip `pulumi new` — scaffolding is proven in run-pod-template-nodejs.sh;
# this test's job is execution PORTABILITY). Substitute the template placeholders in place.
for f in Pulumi.yaml index.js package.json; do
  sed -i.bak -e "s/\${PROJECT}/$PROJECT_NAME/g" \
             -e "s/\${DESCRIPTION}/a remotely-executed OCI nodejs program/g" \
             "$WORK/stage/$f"
  rm -f "$WORK/stage/$f.bak"
done
# The file backend (and PULUMI_HOME) live inside the workspace volume, so state stays
# remote. Pre-create the dir so it rides in the seed tar.
mkdir -p "$WORK/stage/.pulumi-pod"

docker -H "$REMOTE" volume create "$WSVOL" >/dev/null
tar -c -C "$WORK/stage" . \
  | docker -H "$REMOTE" run --rm -i -v "$WSVOL":/workspace alpine \
      sh -c 'tar -x -C /workspace'

# ---------------------------------------------------------------------------
# Run the pod entirely on the remote daemon. Note what is NOT here vs. the local
# template test: NO host bind mounts at all. The workspace is the remote volume; the
# docker socket is the remote daemon's own (so the engine orchestrates dind); stdio
# attaches over the TCP connection to dind.
# ---------------------------------------------------------------------------
echo "==> creating the pod network on the remote daemon"
docker -H "$REMOTE" network create --label "$POD_LABEL" "$NET" >/dev/null

echo "==> running the pod on the remote daemon: up + output (engine, build, and program all run remotely)"
docker -H "$REMOTE" run --rm -i \
  --privileged \
  --network "$NET" \
  --name "$ENGINE_NAME" \
  --hostname "$ENGINE_NAME" \
  --label "$POD_LABEL" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$WSVOL":/workspace \
  -w /workspace \
  -e PULUMI_HOME=/workspace/.pulumi-pod \
  -e PULUMI_POD_MODE=true \
  -e PULUMI_POD_NETWORK="$NET" \
  -e PULUMI_POD_ADVERTISE_HOST="$ENGINE_NAME" \
  -e PULUMI_POD_ID="$POD_ID" \
  -e PULUMI_BACKEND_URL=file:///workspace/.pulumi-pod \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack init '"$STACK"'
    pulumi up --yes --skip-preview --stack '"$STACK"'
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack '"$STACK"')"
  ' \
  2>&1 | tee "$WORK/run.log"

# ---------------------------------------------------------------------------
# Assertions.
#
# The strongest argument is structural and implicit in a passing run: $WSVOL exists
# ONLY on the remote daemon, and the engine's source comes solely from it. Had -H
# $REMOTE silently not taken effect, the engine would have run on the host daemon and
# mounted a fresh, empty local volume of that name — and `up` would have failed on "no
# Pulumi.yaml". So a successful greeting is itself proof the pod ran on the remote. The
# explicit checks below corroborate it.
# ---------------------------------------------------------------------------
echo "==> asserting the program image was BUILT on the remote daemon (not the host)"
if ! grep -q "oci: building program image in builder docker:cli" "$WORK/run.log"; then
  echo "!! no remote program-image build in the log — the build did not run in the pod"
  exit 1
fi

echo "==> asserting the host daemon never carried this pod (it all ran remotely)"
# Every pod docker command above targeted $REMOTE; the host should have NO pod-labelled
# resources. (The dind container itself is on the host, but it is not pod-labelled.)
LOCAL_POD="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
if [ -n "$LOCAL_POD" ]; then
  echo "!! the host daemon ran pod containers — execution was not fully remote"
  docker ps -a --filter "label=$POD_LABEL"
  exit 1
fi

echo "==> asserting the remotely-built program ran and returned its output"
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/run.log" | head -1)"
if [ "$GREETING" != "$EXPECTED" ]; then
  echo "!! unexpected program output: '${GREETING:-<empty>}' (wanted '$EXPECTED')"
  exit 1
fi
echo "    program output = $GREETING"
echo "==> remote-execution smoke test PASS — the engine, the program-image build, and the"
echo "    program all ran on a remote daemon reached only over TCP, with the host's source"
echo "    carried over as a seeded volume and NO host filesystem mounted into the pod"
