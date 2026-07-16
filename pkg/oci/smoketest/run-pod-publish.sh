#!/usr/bin/env bash
#
# Publish->consume acceptance for the identity & publishing design
# (oci-design/package-identity-and-publishing.md). Model under test: the oci:// ref
# recorded by `pulumi package add oci://<ref>` is SELF-LOCATING IDENTITY. It travels
# schema.PluginDownloadURL -> generated SDK default opts -> RegisterResource ->
# provider descriptor -> containerHost.imageFor, where resolution is layered:
#   knob set   -> identity from the pin, location from the knob
#                 (<knob>/<org>/pulumi-provider-<name>:v<version>)
#   knob unset -> the pin verbatim (its own host is the default route)
#   no pin     -> convention under the default org (<knob>/pulumi/pulumi-provider-…)
#
# Scenario (all provider images come through router-proxy endpoints):
#   1. build the dev engine image (branch CLI + oci host + delegate language hosts)
#   2. start router #1: org namespaces are a read-write publish target (embedded
#      registry); pulumi/pulumi-provider-* is read-only, synthesized from released
#      binaries on pull
#   3. `pulumi package publish <component dir> --registry oci://<router#1>
#      --publisher spikeorg` — the WELDED path: publish drives the package's own
#      self-described build (the same one `pulumi install` runs), boots the built
#      image to read its schema (verify by running), checks the reported identity
#      against the manifest's claim, and pushes the ORG ref
#      (localhost:5064/spikeorg/pulumi-provider-greeting:v0.1.0). All local tags
#      are then removed — consumption must be honest.
#      3b. the same for a POLICY PACK: publish builds policy-pack-node/ (its
#      PulumiPolicy.yaml declares name/version + build), verifies via
#      GetAnalyzerInfo, pushes pulumi-policy-oci-policy-smoke, and a preview
#      consumes it via --policy-pack oci://<ref> (pinned resolution)
#   4. `pulumi package add oci://<org ref>` in a fresh Go consumer project: assert
#      the pin lands in Pulumi.yaml AND in the generated SDK's PluginDownloadURL
#   5. `pulumi up` with the registry knob as the ONLY registry config: greeting
#      resolves knob-recomputed from its pin's identity, random resolves by
#      convention under the default org — one endpoint serves both
#   6. ZERO-CONFIG PROBE: unset the knob; a fresh up must resolve greeting from the
#      pin's own host (pull with no registry configured — the pinned-pull path)
#   7. MIGRATION PROBE: mirror greeting to router #2, flip ONLY the knob; a fresh up
#      must pull everything from router #2 with Pulumi.yaml sha256-identical and the
#      existing RandomPet NOT replaced (same pet name)
#   8. OFFLINE PROBE: stop both routers; a fresh up and a destroy must work from the
#      local store alone — no pulls
#
# Usage: bash run-pod-publish.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-publish"
PROGRAM_DIR="$SMOKE_DIR/program-publish"
COMPONENT_DIR="$SMOKE_DIR/component-greeter"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$(cd "$SMOKE_DIR/../../.." && pwd)"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

COMPONENT_PKG="greeting"
COMPONENT_VERSION="0.1.0"
ORG="spikeorg"
RANDOM_PKG="random"
RANDOM_VERSION="4.21.0"

# Failed deployments leak the program container (task: unreaped by the language
# host), which collides with the next run of the same pod id — so each phase runs
# under its own pod id. The network is shared; containers are labeled per phase.
POD_BASE="publish-$$"
NET="pulumi-pod-$POD_BASE"
POD_ID="$POD_BASE" # engine_run reads this; phases override it
# Deliberately NOT pulumi-cli-oci:latest — a concurrent experiment shares the daemon.
ENGINE_IMAGE="pulumi-cli-oci:proto-publish"
PROGRAM_IMAGE="oci-smoke-publish:proto"
STACK="dev"

# Two router instances: unique names and ports (5005 belongs to the shared wrapper
# proxy; 5061-5063 to the bake-off prototype worktrees).
REG1_PORT=5064
REG2_PORT=5065
REG1="localhost:$REG1_PORT"
REG2="localhost:$REG2_PORT"
PROXY1_NAME="proto-publish-proxy-1"
PROXY2_NAME="proto-publish-proxy-2"

# The working tag the welded build produces (BuildPackage's host-unqualified
# convention ref) — NOTE: shared spelling with the mlc smoke tests, which build the
# same component; they rebuild it themselves, so removing it here is safe.
WORK_TAG="pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"
# The discriminating builder component-greeter's PulumiPlugin.yaml names.
BUILDER_IMAGE="oci-smoke-builder:latest"
# The policy pack under publish, and its refs.
POLICY_DIR="$SMOKE_DIR/policy-pack-node"
POLICY_PKG="oci-policy-smoke"
POLICY_VERSION="1.0.0"
POLICY_WORK_TAG="pulumi/pulumi-policy-$POLICY_PKG:v$POLICY_VERSION"
POLICY_ORG_REF="$REG1/$ORG/pulumi-policy-$POLICY_PKG:v$POLICY_VERSION"
# The ORG-namespaced ref the component is published under — the pin under test.
ORG_REF="$REG1/$ORG/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"
OCI_SOURCE="oci://$ORG_REF"
# The same identity relocated to router #2 (what the migration probe must compute).
ORG_REF2="$REG2/$ORG/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"
# The convention refs the engine must compute for the released random provider:
# always org-namespaced, under the default org.
RANDOM_REF1="$REG1/pulumi/pulumi-provider-$RANDOM_PKG:v$RANDOM_VERSION"
RANDOM_REF2="$REG2/pulumi/pulumi-provider-$RANDOM_PKG:v$RANDOM_VERSION"
RANDOM_LOCAL="pulumi/pulumi-provider-$RANDOM_PKG:v$RANDOM_VERSION"
# The ref greeting would resolve to if its pin's identity were LOST and it fell back
# to default-org convention — it must never appear in any log.
GREETING_DEFAULTED_REF1="$REG1/pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"
GREETING_DEFAULTED_REF2="$REG2/pulumi/pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project" "$WORK/state" "$WORK/consumer"

cleanup() {
  local leftovers
  for phase in "$POD_BASE" "$POD_BASE-pub" "$POD_BASE-add" "$POD_BASE-up" "$POD_BASE-pol" "$POD_BASE-polup" "$POD_BASE-zc" "$POD_BASE-mig" "$POD_BASE-off" "$POD_BASE-dst"; do
    leftovers="$(docker ps -aq --filter "label=com.pulumi.pod=$phase" 2>/dev/null || true)"
    [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
    docker volume rm "pulumi-pod-$phase-vol-workspace" >/dev/null 2>&1 || true
  done
  docker rm -f "$PROXY1_NAME" "$PROXY2_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  # Only refs this run owns (5064/5065-qualified, this run's program image, and the
  # host-unqualified random tag the zero-config probe creates).
  docker image rm -f "$ORG_REF" "$ORG_REF2" "$RANDOM_REF1" "$RANDOM_REF2" "$RANDOM_LOCAL" \
    "$GREETING_DEFAULTED_REF1" "$GREETING_DEFAULTED_REF2" "$PROGRAM_IMAGE" "$WORK_TAG" \
    "$POLICY_WORK_TAG" "$POLICY_ORG_REF" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run publish test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling the registry-proxy (linux/$GOARCH)"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )

start_proxy() { # name port
  docker rm -f "$1" >/dev/null 2>&1 || true
  docker run -d --name "$1" -p "$2:5000" \
    -e PROXY_TARGET_ARCH="$GOARCH" \
    -v "$WORK/registry-proxy-linux":/registry-proxy:ro \
    alpine sh -c 'apk add --no-cache ca-certificates >/dev/null 2>&1 && exec /registry-proxy' >/dev/null
  for _ in $(seq 1 30); do
    curl -sf "http://localhost:$2/v2/" >/dev/null 2>&1 && return 0
    sleep 0.5
  done
  echo "!! router proxy $1 did not come up on localhost:$2"
  docker logs "$1" 2>&1 | tail -20
  return 1
}

echo "==> starting router #1 ($PROXY1_NAME on $REG1: org publish target + synthesized providers)"
start_proxy "$PROXY1_NAME" "$REG1_PORT"
echo "    router #1 up"

echo "==> building the discriminating builder image ($BUILDER_IMAGE — component-greeter's build.image)"
docker buildx build --builder "$BUILDER" --load -t "$BUILDER_IMAGE" -f "$SMOKE_DIR/Dockerfile.builder" "$SMOKE_DIR"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

# Run a CLI command inside the engine container, in pod mode. KNOB is the registry
# configuration for the run — the ONLY location input; "" means unset. POD_ID is
# phase-scoped (see above).
KNOB="$REG1"
engine_run() {
  docker run --rm -i \
    --privileged \
    --network "$NET" \
    --name "pulumi-pod-$POD_ID-engine" \
    --hostname "pulumi-pod-$POD_ID-engine" \
    --label "com.pulumi.pod=$POD_ID" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$WORK/project":/project \
    -v "$WORK/state":/state \
    -v "$COMPONENT_DIR":/packages/greeter:ro \
    -v "$POLICY_DIR":/packages/policy:ro \
    -w /project \
    -e PULUMI_POD_MODE=true \
    -e PULUMI_POD_NETWORK="$NET" \
    -e PULUMI_POD_ADVERTISE_HOST="pulumi-pod-$POD_ID-engine" \
    -e PULUMI_POD_ID="$POD_ID" \
    -e PULUMI_POD_PLUGIN_REGISTRY="$KNOB" \
    -e PULUMI_BACKEND_URL=file:///state \
    -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
    -e STACK="$STACK" \
    --entrypoint sh \
    "$ENGINE_IMAGE" \
    -c "$1"
}

###############################################################################
# publish (welded): the package DIRECTORY is the unit — publish builds it via its
# own self-described build, verifies the artifact against the manifest claim, and
# pushes the identity-derived org ref.
###############################################################################
POD_ID="$POD_BASE-pub"
KNOB="$REG1"
echo "==> pulumi package publish /packages/greeter --registry oci://$REG1 --publisher $ORG (welded: build + verify + push)"
engine_run "pulumi package publish /packages/greeter --registry 'oci://$REG1' --publisher '$ORG'" 2>&1 | tee "$WORK/publish.log"
if ! grep -q "Building $COMPONENT_PKG (v$COMPONENT_VERSION)" "$WORK/publish.log"; then
  echo "!! publish did not drive the package's own build"
  exit 1
fi
if ! grep -q "Published $ORG/$COMPONENT_PKG@$COMPONENT_VERSION to oci://$ORG_REF" "$WORK/publish.log"; then
  echo "!! publish did not derive the org ref from the identity the running image reported"
  exit 1
fi
echo "    publish built the package, verified it by running it, and pushed $ORG_REF"

echo "==> removing local greeting/random image tags so consumption is honest"
docker image rm -f "$WORK_TAG" "$ORG_REF" "$GREETING_DEFAULTED_REF1" "$RANDOM_REF1" >/dev/null 2>&1 || true
for ref in "$WORK_TAG" "$ORG_REF" "$GREETING_DEFAULTED_REF1" "$RANDOM_REF1"; do
  if docker image inspect "$ref" >/dev/null 2>&1; then
    echo "!! $ref still present locally — the consumption test would not be honest"
    exit 1
  fi
done
echo "    published to router #1's backend; nothing greeting/random remains local"

###############################################################################
# package add: the pin is written and baked into the generated SDK.
###############################################################################
POD_ID="$POD_BASE-add"
echo "==> pulumi package add $OCI_SOURCE (schema-from-image + delegated Go codegen + pin)"
engine_run "pulumi package add '$OCI_SOURCE'" 2>&1 | tee "$WORK/add.log"

echo "==> asserting the pin was recorded in Pulumi.yaml"
if ! grep -q "$OCI_SOURCE" "$WORK/project/Pulumi.yaml"; then
  echo "!! the oci:// pin was not written into Pulumi.yaml"
  cat "$WORK/project/Pulumi.yaml"
  exit 1
fi
echo "--- Pulumi.yaml packages pin (verbatim) ---"
sed -n '/^packages:/,$p' "$WORK/project/Pulumi.yaml"
echo "-------------------------------------------"
PULUMI_YAML_SHA="$(shasum -a 256 "$WORK/project/Pulumi.yaml" | awk '{print $1}')"

echo "==> asserting the generated SDK bakes the oci:// ref as its PluginDownloadURL (the codegen leg)"
if ! grep -rq "pulumi.PluginDownloadURL(\"$OCI_SOURCE\")" "$WORK/project/sdks/$COMPONENT_PKG"; then
  echo "!! the generated SDK does not carry the oci:// pin as a PluginDownloadURL default"
  grep -rn "PluginDownloadURL" "$WORK/project/sdks/$COMPONENT_PKG" || true
  exit 1
fi
echo "    generated SDK defaults include pulumi.PluginDownloadURL(\"$OCI_SOURCE\")"

# The add pulled the image to read its schema; remove it again so the up must
# resolve the pin from the router, not from a local leftover.
echo "==> removing the greeting image pulled during add (up must pull via the resolver)"
docker image rm -f "$ORG_REF" >/dev/null 2>&1 || true

###############################################################################
# Build the consumer program on the host against the generated SDK, bake its image.
###############################################################################
echo "==> assembling consumer program against the generated SDK"
SDK_DIR="$WORK/project/sdks/$COMPONENT_PKG"
if [ ! -f "$SDK_DIR/go.mod" ]; then
  echo "!! generated SDK has no go.mod — cannot build the consumer against it"
  find "$WORK/project/sdks" -type f | head -20
  exit 1
fi
SDK_MODULE="$(awk '$1=="module"{print $2; exit}' "$SDK_DIR/go.mod")"
echo "    generated SDK module: $SDK_MODULE (at sdks/$COMPONENT_PKG)"
if [ "$SDK_MODULE" != "example.com/pulumi-greeting/sdk/go" ]; then
  echo "!! unexpected SDK module path; program-publish/main.go imports example.com/pulumi-greeting/sdk/go/greeting"
  exit 1
fi
cp "$PROGRAM_DIR/main.go" "$WORK/consumer/"
cat > "$WORK/consumer/go.mod" <<EOF
module oci-publish-consumer

go 1.25

replace github.com/pulumi/pulumi/sdk/v3 => $REPO_ROOT/sdk

replace $SDK_MODULE => $SDK_DIR
EOF
( cd "$WORK/consumer" && GOWORK=off go mod tidy ) >"$WORK/tidy.log" 2>&1 || {
  echo "!! go mod tidy for the consumer failed"; tail -20 "$WORK/tidy.log"; exit 1; }
echo "==> cross-compiling consumer program (linux/$GOARCH)"
( cd "$WORK/consumer" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/consumer/program-linux" . )

echo "==> building program image $PROGRAM_IMAGE"
cat > "$WORK/consumer/Dockerfile" <<'EOF'
FROM alpine:3
COPY program-linux /program
ENTRYPOINT ["/program"]
EOF
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$WORK/consumer/Dockerfile" "$WORK/consumer"

pet_of() { # extracts the RandomPet name from a SMOKE message line in a log
  sed -n 's/.*(pet: \([^)]*\)).*/\1/p' "$1" | head -1
}

###############################################################################
# up: greeting knob-recomputed from its pin, random by default-org convention.
###############################################################################
POD_ID="$POD_BASE-up"
KNOB="$REG1"
echo "==> pulumi up (registry knob $REG1 is the ONLY registry configuration)"
engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select --create "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE message=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' 2>&1 | tee "$WORK/up.log"

echo "==> asserting greeting resolved from its pin's identity through the knob"
if ! grep -q "oci: provider $COMPONENT_PKG resolved by its oci:// pin: $ORG_REF" "$WORK/up.log"; then
  echo "!! the engine did not resolve greeting from its oci:// pin"
  exit 1
fi
if ! grep -q "oci: installed plugin $ORG_REF by pulling its image" "$WORK/up.log"; then
  echo "!! greeting's image was not installed by pull from the org namespace"
  exit 1
fi
if grep -q "$GREETING_DEFAULTED_REF1" "$WORK/up.log"; then
  echo "!! greeting fell back to default-org convention — its pin's identity was lost"
  exit 1
fi
echo "    greeting: pin identity -> $ORG_REF (pulled from router #1's org namespace)"

echo "==> asserting random resolved by convention under the default org (pulumi/)"
if ! grep -q "oci: installed plugin $RANDOM_REF1 by pulling its image" "$WORK/up.log"; then
  echo "!! random was not installed by pulling its default-org convention ref"
  exit 1
fi
docker logs "$PROXY1_NAME" >"$WORK/proxy1.log" 2>&1
if ! grep -q "synthesizing pulumi/pulumi-provider-$RANDOM_PKG" "$WORK/proxy1.log"; then
  echo "!! router #1 did not synthesize the random provider image"
  exit 1
fi
echo "    random: convention -> $RANDOM_REF1 (synthesized by router #1 from the released binary)"

MESSAGE="$(sed -n 's/.*SMOKE message=<<\(.*\)>>.*/\1/p' "$WORK/up.log" | head -1)"
case "$MESSAGE" in
  *"(pet: "*) echo "    message = $MESSAGE" ;;
  *) echo "!! stack output missing or unexpected: '${MESSAGE:-<empty>}'"; exit 1 ;;
esac
PET_BEFORE="$(pet_of "$WORK/up.log")"

###############################################################################
# policy pack: publish (build + GetAnalyzerInfo verify + push) and consume by
# --policy-pack oci://<ref> (pinned resolution through the router).
###############################################################################
POD_ID="$POD_BASE-pol"
KNOB="$REG1"
echo "==> pulumi package publish /packages/policy --registry oci://$REG1 --publisher $ORG (policy pack)"
engine_run "pulumi package publish /packages/policy --registry 'oci://$REG1' --publisher '$ORG'" 2>&1 | tee "$WORK/publish-policy.log"
if ! grep -q "Building $POLICY_PKG (v$POLICY_VERSION)" "$WORK/publish-policy.log"; then
  echo "!! policy publish did not drive the pack's own build"
  exit 1
fi
if ! grep -q "Published $ORG/$POLICY_PKG@$POLICY_VERSION (policy pack) to oci://$POLICY_ORG_REF" "$WORK/publish-policy.log"; then
  echo "!! policy publish did not derive the org ref from what the running pack reported"
  exit 1
fi
echo "    pack built, verified via GetAnalyzerInfo, pushed $POLICY_ORG_REF"

echo "==> consuming the published pack: preview --policy-pack oci://$POLICY_ORG_REF (honest pull)"
docker image rm -f "$POLICY_WORK_TAG" "$POLICY_ORG_REF" >/dev/null 2>&1 || true
POD_ID="$POD_BASE-polup"
engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select "$STACK"
    pulumi preview --policy-pack "oci://'"$POLICY_ORG_REF"'" --stack "$STACK"
  ' 2>&1 | tee "$WORK/preview-policy.log"
if ! grep -q "oci: policy pack .* resolved by its oci:// pin: $POLICY_ORG_REF" "$WORK/preview-policy.log"; then
  echo "!! the pack was not resolved from its oci:// pin"
  exit 1
fi
if ! grep -q "oci: installed plugin $POLICY_ORG_REF by pulling its image" "$WORK/preview-policy.log"; then
  echo "!! the published pack was not installed by pulling its image"
  exit 1
fi
if ! grep -q "oci: policy pack .* running as container" "$WORK/preview-policy.log"; then
  echo "!! the pack did not run as an analyzer container"
  exit 1
fi
echo "    published pack pulled by pin and engaged in the preview"

###############################################################################
# ZERO-CONFIG PROBE: no knob — the pin's own host is the default route.
###############################################################################
POD_ID="$POD_BASE-zc"
KNOB=""
echo "==> ZERO-CONFIG PROBE: knob unset; greeting must pull via the pin's own host"
# Greeting must be absent (an honest pull); random has no pin and no knob, so it
# must resolve from the local store under its host-unqualified default-org name.
docker image rm -f "$ORG_REF" >/dev/null 2>&1 || true
docker tag "$RANDOM_REF1" "$RANDOM_LOCAL"
engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE zc-up=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' 2>&1 | tee "$WORK/up-zeroconfig.log"

if ! grep -q "SMOKE zc-up=<<" "$WORK/up-zeroconfig.log"; then
  echo "!! zero-config up did not complete"
  exit 1
fi
if ! grep -q "oci: provider $COMPONENT_PKG resolved by its oci:// pin: $ORG_REF" "$WORK/up-zeroconfig.log"; then
  echo "!! with no knob, greeting did not resolve to its pin verbatim"
  exit 1
fi
if ! grep -q "oci: installed plugin $ORG_REF by pulling its image" "$WORK/up-zeroconfig.log"; then
  echo "!! greeting was not pulled via its pin with no registry configured (the pinned-pull path)"
  exit 1
fi
echo "    PROBE RESULT: zero-config GREEN — the pin alone located and installed the package"

###############################################################################
# MIGRATION PROBE: mirror to router #2, flip only the knob.
###############################################################################
POD_ID="$POD_BASE-mig"
KNOB="$REG2"
echo "==> MIGRATION PROBE: starting router #2 ($PROXY2_NAME on $REG2) and mirroring greeting"
start_proxy "$PROXY2_NAME" "$REG2_PORT"
docker tag "$ORG_REF" "$ORG_REF2" # the zero-config probe re-pulled it; mirror = tag + push
docker push -q "$ORG_REF2" >/dev/null
echo "==> removing local $REG2 tags so migration pulls are honest"
docker image rm -f "$ORG_REF2" "$RANDOM_REF2" "$GREETING_DEFAULTED_REF2" >/dev/null 2>&1 || true

echo "==> pulumi up with ONLY the knob changed ($REG1 -> $REG2)"
engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE migrated=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' 2>&1 | tee "$WORK/up-migrate.log"

if ! grep -q "oci: installed plugin $ORG_REF2 by pulling its image" "$WORK/up-migrate.log"; then
  echo "!! greeting was not pulled from router #2 — the knob did not relocate the pin's identity"
  exit 1
fi
if ! grep -q "oci: installed plugin $RANDOM_REF2 by pulling its image" "$WORK/up-migrate.log"; then
  echo "!! random was not pulled from router #2"
  exit 1
fi
PULUMI_YAML_SHA_AFTER="$(shasum -a 256 "$WORK/project/Pulumi.yaml" | awk '{print $1}')"
if [ "$PULUMI_YAML_SHA" != "$PULUMI_YAML_SHA_AFTER" ]; then
  echo "!! Pulumi.yaml changed across the migration — the model's central claim failed"
  exit 1
fi
PET_AFTER="$(pet_of "$WORK/up-migrate.log")"
if [ -z "$PET_BEFORE" ] || [ "$PET_BEFORE" != "$PET_AFTER" ]; then
  echo "!! the RandomPet was replaced across the migration (before: '$PET_BEFORE', after: '$PET_AFTER')"
  exit 1
fi
echo "    PROBE RESULT: migration GREEN — one knob change; Pulumi.yaml sha256-identical;"
echo "                  pet '$PET_AFTER' survived un-replaced; both packages pulled from router #2"

###############################################################################
# OFFLINE PROBE: both routers stopped, images local — fresh up + destroy, no pulls.
###############################################################################
POD_ID="$POD_BASE-off"
KNOB="$REG2"
echo "==> OFFLINE PROBE: stopping both routers; fresh up + destroy from the local store"
docker stop "$PROXY1_NAME" "$PROXY2_NAME" >/dev/null

engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select "$STACK"
    pulumi up --yes --skip-preview --stack "$STACK"
    printf "SMOKE offline-up=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' 2>&1 | tee "$WORK/up-offline.log"

if ! grep -q "SMOKE offline-up=<<" "$WORK/up-offline.log"; then
  echo "!! offline up did not complete"
  exit 1
fi
if grep -q "by pulling its image" "$WORK/up-offline.log"; then
  echo "!! offline up pulled — resolution should have been satisfied by the local store"
  exit 1
fi
echo "    offline up GREEN — pin + convention both resolved from the local store, no pulls"

POD_ID="$POD_BASE-dst"
engine_run '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi stack select "$STACK"
    pulumi destroy --yes --stack "$STACK"
  ' 2>&1 | tee "$WORK/destroy-offline.log"

if ! grep -qE "deleted" "$WORK/destroy-offline.log" || grep -qi "error" "$WORK/destroy-offline.log"; then
  echo "!! offline destroy did not go green"
  tail -20 "$WORK/destroy-offline.log"
  exit 1
fi
echo "    offline destroy GREEN — providers rebooted from state/local images"

echo "==> publish smoke test PASS — a component published to an org namespace was consumed"
echo "    via its self-locating oci:// pin (knob-recomputed, zero-config, migrated with one"
echo "    knob flip, and offline), while a released provider resolved by convention under"
echo "    the default org — every image through the router endpoints"
