#!/usr/bin/env bash
#
# CRI build+publish smoke test — the full kaniko->CRI run. The engine runs
# `pulumi package build` inside a CRI PodSandbox, and the whole build-contract sink
# flows through the criPodManager for the first time end-to-end:
#
#   RunToCompletion   — the kaniko builder runs as a one-shot CRI container
#   source-reaching   — kaniko inherits the engine's mounts (the --volumes-from
#                       analog) to read the package source AND write the OCI layout
#                       to the shared /pulumi-build scratch (the outbound round-trip)
#   ImportImage       — the engine pushes the layout to a registry in-process (ggcr
#                       remote.Write) and PullImage's it back into the k8s.io store
#
# kaniko (not `docker build`) is the builder: a daemonless executor that needs no
# second ambient dependency, so "a container runtime" stays the sole dependency —
# the thesis-preserving build, and the forcing function for the runtime-neutral
# build contract (it cannot reach a daemon socket to cheat).
#
# The sink is `plainregistry` (a bare ggcr registry.New()), NOT the synthesizing
# registry-proxy: the build ref is ProviderImageRef -> pulumi/pulumi-provider-<name>,
# a namespace the proxy reserves read-only (it 405s the push). That collision is a
# real finding; the plain registry sidesteps it so the CRI build+publish mechanism
# can be proven on its own. The registry sits at 10.88.0.1:5000 — the cri0 gateway,
# reachable from BOTH the pod netns (engine push) and the host netns (containerd
# pull) — and containerd pulls it over plain-http via a certs.d hosts.toml.
#
# Prerequisites (see the CRI spike kit ~/scratch/2026-07-18_cri-podmanager/):
#   - docker daemon; the `crienv` container (kubelet-free containerd+CRI)
#   - crienv's containerd registry.config_path set to /etc/containerd/certs.d
#     (the (c) probe set this; survives restart)
#   - kaniko (gcr.io/kaniko-project/executor:debug) loaded into crienv's k8s.io store
#   - the plainregistry binary cross-compiled at the spike kit
#
# Usage: run-pod-cri-build.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh"
PACKAGE_DIR="$SMOKE_DIR/package-kaniko"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

CRIENV=crienv
SPIKE_KIT="$HOME/scratch/2026-07-18_cri-podmanager"
ENGINE_IMAGE="pulumi-cli-oci:latest"
KANIKO_IMAGE="gcr.io/kaniko-project/executor:debug"
POD_ID="cri-build-$$"
LOGDIR="/var/log/pods/$POD_ID"
VOLDIR="/var/lib/pulumi-pod/$POD_ID/volumes"
BUILDDIR_HOST="/cri-build/$POD_ID/pulumi-build"   # PULUMI_POD_BUILD_DIR — the layout round-trip
META_HOST="/cri-build/$POD_ID/meta"               # engine-id injection (create-time chicken/egg)
PROJ_HOST="/cri-build/$POD_ID/project"            # the package source the builder inherits

# The registry sink: both-netns address (cri0 gateway); the build ref lands here.
REG_ADDR="10.88.0.1:5000"
CERTS_D=/etc/containerd/certs.d
COMPONENT_PKG="kanikoprobe"
COMPONENT_VERSION="0.1.0"
# ProviderImageRef(registry, name, version) = <reg>/pulumi/pulumi-provider-<name>:v<version>
BUILT_REF="$REG_ADDR/pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"

WORK="$(mktemp -d)"
SB=""
cleanup() {
  echo "== cleanup =="
  if [ -n "$SB" ]; then
    docker exec "$CRIENV" crictl stopp "$SB" >/dev/null 2>&1 || true
    docker exec "$CRIENV" crictl rmp -f "$SB" >/dev/null 2>&1 || true
    echo "   reaped sandbox $SB"
  fi
  docker exec "$CRIENV" rm -rf "/cri-build/$POD_ID" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

# ── preflight ──────────────────────────────────────────────────────────────
docker info >/dev/null 2>&1 || { echo "!! docker daemon not available"; exit 1; }
docker exec "$CRIENV" crictl version >/dev/null 2>&1 || { echo "!! crienv not running — see the CRI spike kit"; exit 1; }
docker exec "$CRIENV" crictl images 2>/dev/null | grep -q kaniko || {
  echo "!! kaniko not loaded in crienv. Load it: docker pull $KANIKO_IMAGE && docker save $KANIKO_IMAGE | docker exec -i $CRIENV ctr -n k8s.io images import -"; exit 1; }
[ -x "$SPIKE_KIT/plainregistry" ] || {
  echo "!! plainregistry binary missing. Build it: (cd $SMOKE_DIR/registry-proxy && GOOS=linux GOARCH=$GOARCH go build -o $SPIKE_KIT/plainregistry ./plainregistry)"; exit 1; }

# ── build + load the engine image ───────────────────────────────────────────
build_engine_image
echo "==> loading $ENGINE_IMAGE into crienv k8s.io store"
docker save "$ENGINE_IMAGE" | docker exec -i "$CRIENV" ctr -n k8s.io images import - >/dev/null

# ── start the plain registry sink on :5000 (kill any squatter first) ────────
docker cp "$SPIKE_KIT/plainregistry" "$CRIENV:/usr/local/bin/plainregistry"
docker exec "$CRIENV" pkill -f 'registry-proxy|plainregistry' 2>/dev/null || true
sleep 1
docker exec -d "$CRIENV" sh -c 'PLAIN_REGISTRY_ADDR=:5000 /usr/local/bin/plainregistry >/tmp/plainreg.log 2>&1'
for i in $(seq 1 15); do
  docker exec "$CRIENV" sh -c 'curl -sf http://127.0.0.1:5000/v2/ >/dev/null 2>&1' && break; sleep 1
done
# Discriminator: the sink must ACCEPT a provider-namespace upload (202), where the
# synthesizing proxy would 405 — proving plainregistry is really the server on :5000.
UP="$(docker exec "$CRIENV" sh -c "curl -s -X POST -o /dev/null -w '%{http_code}' http://127.0.0.1:5000/v2/pulumi/pulumi-provider-$COMPONENT_PKG/blobs/uploads/")"
[ "$UP" = "202" ] || { echo "!! sink on :5000 returned $UP (want 202) — not plainregistry"; docker exec "$CRIENV" cat /tmp/plainreg.log; exit 1; }
echo "==> plainregistry up on :5000 (provider-namespace upload -> $UP)"

# ── remove any prior built image so the run must produce it ─────────────────
docker exec "$CRIENV" crictl rmi "$BUILT_REF" >/dev/null 2>&1 || true

# ── stage the package source + scratch dirs inside crienv ───────────────────
docker exec "$CRIENV" sh -c "rm -rf /cri-build/$POD_ID && mkdir -p '$PROJ_HOST/package-kaniko' '$BUILDDIR_HOST' '$META_HOST' '$LOGDIR' '$VOLDIR'"
docker cp "$PACKAGE_DIR/PulumiPlugin.yaml" "$CRIENV:$PROJ_HOST/package-kaniko/PulumiPlugin.yaml"
docker cp "$PACKAGE_DIR/Dockerfile"        "$CRIENV:$PROJ_HOST/package-kaniko/Dockerfile"
docker cp "$PACKAGE_DIR/marker.txt"        "$CRIENV:$PROJ_HOST/package-kaniko/marker.txt"

# ── create the pod sandbox (brings up cri0 / 10.88.0.1) ─────────────────────
for p in $(docker exec "$CRIENV" crictl pods --name "$POD_ID" -q 2>/dev/null); do
  docker exec "$CRIENV" crictl stopp "$p" >/dev/null 2>&1 || true
  docker exec "$CRIENV" crictl rmp -f "$p" >/dev/null 2>&1 || true
done
docker exec "$CRIENV" sh -c "cat > /tmp/$POD_ID-sandbox.json <<JSON
{
  \"metadata\": { \"name\": \"$POD_ID\", \"namespace\": \"pulumi\", \"uid\": \"$POD_ID\", \"attempt\": 1 },
  \"log_directory\": \"$LOGDIR\",
  \"linux\": { \"security_context\": { \"namespace_options\": { \"network\": 0 } } }
}
JSON"
SB="$(docker exec "$CRIENV" crictl runp /tmp/$POD_ID-sandbox.json)"
echo "==> sandbox: $SB"

# ── (re)assert the 10.88.0.1:5000 plain-http hosts.toml (hot-reloads) ───────
docker exec "$CRIENV" sh -c "mkdir -p '$CERTS_D/$REG_ADDR' && cat > '$CERTS_D/$REG_ADDR/hosts.toml' <<TOML
server = \"http://$REG_ADDR\"
[host.\"http://$REG_ADDR\"]
  capabilities = [\"pull\", \"resolve\"]
TOML"

# ── the engine entrypoint: inject the engine's own container id, then build ──
# On CRI os.Hostname() is the sandbox pause id, so source-reaching needs the engine's
# OWN container id via PULUMI_POD_ENGINE_CONTAINER_ID. The id is unknown until after
# `crictl create` (env is fixed at create), so the wrapper writes it to a mounted file
# post-create/pre-start and this entrypoint exports it. No engine code change.
docker exec "$CRIENV" sh -c "cat > $PROJ_HOST/engine-run.sh <<'SCRIPT'
#!/bin/sh
set -e
export PULUMI_POD_ENGINE_CONTAINER_ID=\"\$(cat /pod-meta/engine-id)\"
echo \"engine container id: \$PULUMI_POD_ENGINE_CONTAINER_ID\"
pulumi package build package-kaniko
SCRIPT
chmod +x $PROJ_HOST/engine-run.sh"

# ── create the engine container in the sandbox ──────────────────────────────
docker exec "$CRIENV" sh -c "cat > /tmp/$POD_ID-engine.json <<JSON
{
  \"metadata\": { \"name\": \"engine\", \"attempt\": 0 },
  \"image\": { \"image\": \"$ENGINE_IMAGE\" },
  \"command\": [\"/bin/sh\"],
  \"args\": [\"/project/engine-run.sh\"],
  \"working_dir\": \"/project\",
  \"envs\": [
    { \"key\": \"PULUMI_POD_MODE\",           \"value\": \"true\" },
    { \"key\": \"PULUMI_POD_RUNTIME\",        \"value\": \"cri\" },
    { \"key\": \"PULUMI_POD_SANDBOX_ID\",     \"value\": \"$SB\" },
    { \"key\": \"PULUMI_POD_LOG_DIR\",        \"value\": \"$LOGDIR\" },
    { \"key\": \"PULUMI_POD_ID\",             \"value\": \"$POD_ID\" },
    { \"key\": \"PULUMI_POD_VOLUME_DIR\",     \"value\": \"$VOLDIR\" },
    { \"key\": \"PULUMI_POD_BUILD_DIR\",      \"value\": \"/pulumi-build\" },
    { \"key\": \"PULUMI_POD_PLUGIN_REGISTRY\", \"value\": \"$REG_ADDR\" }
  ],
  \"mounts\": [
    { \"host_path\": \"/run/containerd/containerd.sock\", \"container_path\": \"/run/containerd/containerd.sock\" },
    { \"host_path\": \"$LOGDIR\",       \"container_path\": \"$LOGDIR\" },
    { \"host_path\": \"$VOLDIR\",       \"container_path\": \"$VOLDIR\" },
    { \"host_path\": \"$PROJ_HOST\",    \"container_path\": \"/project\" },
    { \"host_path\": \"$BUILDDIR_HOST\", \"container_path\": \"/pulumi-build\" },
    { \"host_path\": \"$META_HOST\",    \"container_path\": \"/pod-meta\" }
  ],
  \"log_path\": \"engine_0.log\"
}
JSON"
ENGINE_ID="$(docker exec "$CRIENV" crictl create "$SB" /tmp/$POD_ID-engine.json /tmp/$POD_ID-sandbox.json)"
echo "==> engine container: $ENGINE_ID"

# ── inject the engine id into the mounted meta dir, THEN start ──────────────
docker exec "$CRIENV" sh -c "echo '$ENGINE_ID' > $META_HOST/engine-id"
docker exec "$CRIENV" crictl start "$ENGINE_ID"

# ── wait for the engine to finish ──────────────────────────────────────────
echo "==> waiting for engine (package build) to exit..."
TIMEOUT=180; ELAPSED=0
while true; do
  STATE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.state}}' "$ENGINE_ID" 2>/dev/null || echo unknown)"
  [ "$STATE" = "CONTAINER_EXITED" ] && break
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "!! engine did not exit within ${TIMEOUT}s"; docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 || true; exit 1
  fi
  sleep 3; ELAPSED=$((ELAPSED + 3))
done
EXIT_CODE="$(docker exec "$CRIENV" crictl inspect --output go-template --template '{{.status.exitCode}}' "$ENGINE_ID" 2>/dev/null || echo -1)"

echo "== engine logs =="
docker exec "$CRIENV" crictl logs "$ENGINE_ID" 2>&1 | tee "$WORK/engine.log"
echo "==> engine exited with code $EXIT_CODE after ~${ELAPSED}s"

# ── assertions ─────────────────────────────────────────────────────────────
echo "==> checking results"
if [ "$EXIT_CODE" != "0" ]; then
  echo "!! engine exited $EXIT_CODE (expected 0) — CRI build+publish FAIL"
  echo "   (this is the first full weld — the point is to see WHAT broke; check the log above)"
  exit 1
fi

# 1. the build ran in the kaniko builder container (RunToCompletion), not in-process
if ! grep -q "Building $COMPONENT_PKG .*in $KANIKO_IMAGE" "$WORK/engine.log"; then
  echo "!! build did not run in the kaniko builder ($KANIKO_IMAGE)"; exit 1
fi
# 2. the engine loaded the OCI layout and imported it (source-reaching + round-trip + ImportImage)
if ! grep -q "Importing $COMPONENT_PKG from OCI layout" "$WORK/engine.log"; then
  echo "!! engine did not reach the ImportImage sink — no layout was loaded"; exit 1
fi
# 3. package build printed the convention ref on stdout
if ! grep -qx "$BUILT_REF" "$WORK/engine.log"; then
  echo "!! package build did not print the expected ref $BUILT_REF"; exit 1
fi
# 4. THE PUSH (direct): the built image is queryable at the plainregistry sink — proving
#    ImportImage's remote.Write landed it in the registry (not just inferred via the pull).
TAGS="$(docker exec "$CRIENV" sh -c "curl -sf http://127.0.0.1:5000/v2/pulumi/pulumi-provider-$COMPONENT_PKG/tags/list" 2>/dev/null || true)"
if echo "$TAGS" | grep -q "v$COMPONENT_VERSION"; then
  echo "    pushed to sink: $TAGS"
else
  echo "!! $BUILT_REF not found at the plainregistry sink — ImportImage push did not land: ${TAGS:-<no response>}"; exit 1
fi
# 5. THE PULL: ImportImage's PullImage landed the freshly-built image in the k8s.io store.
#    (On CRI a filesystem layout can only enter the store via a registry pull — there is no
#    direct-load path — so a fresh ref here, after `crictl rmi` at start, closes the chain.)
if docker exec "$CRIENV" crictl images | grep -q "pulumi-provider-$COMPONENT_PKG"; then
  echo "    $BUILT_REF present in k8s.io: $(docker exec "$CRIENV" crictl images | grep "pulumi-provider-$COMPONENT_PKG")"
else
  echo "!! $BUILT_REF is not in crienv's k8s.io store — ImportImage push+pull did not complete"; exit 1
fi

echo "==> ✅ CRI build+publish smoke test PASS"
echo "    kaniko built a layout in a one-shot CRI container, the engine imported it via"
echo "    proxy-pull (push to $REG_ADDR + PullImage), and it landed in the k8s.io store."
