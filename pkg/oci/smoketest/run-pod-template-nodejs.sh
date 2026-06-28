#!/usr/bin/env bash
#
# `pulumi new oci-nodejs` template smoke test — scaffold, build, run. Proves the
# scaffolding story end to end: a user runs `pulumi new` against the oci-nodejs template,
# gets a complete project (Pulumi.yaml runtime: oci, an index.js, a Dockerfile they own,
# and an EXPLICIT oci-bootstrap.sh entrypoint — no hidden base-image magic), and `pulumi
# up` builds the program image from that Dockerfile and runs it as a pod container.
#
# The bootstrap is the crux of the template: there is no Pulumi-provided base image baking
# in the entrypoint — the template drops oci-bootstrap.sh into the project, so the user can
# pick any node base image and see exactly what runs. This test exercises that path: the
# program's Dockerfile (FROM node:20-alpine) + the scaffolded bootstrap is the whole image.
#
# Pipeline:
#   1. build the engine image (this branch's CLI + pulumi-language-oci)
#   2. `pulumi new <oci-nodejs template>` into the mounted workspace
#   3. `pulumi up` — the OCI host builds the program image (build.image=docker:cli runs
#      `docker build -q .` on the scaffolded Dockerfile) and runs it on the pod network
#   4. assert the program's exported output came back (the scaffolded program ran)
#
# Usage: run-pod-template-nodejs.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
TEMPLATE_DIR="$SMOKE_DIR/templates/oci-nodejs"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
PROJECT_NAME="oci-new-smoke"
EXPECTED="hello from $PROJECT_NAME, an OCI nodejs program"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project" "$WORK/state"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run template test"
  exit 1
fi

build_engine_image

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

echo "==> pulumi new oci-nodejs -> up -> output (scaffold a project, build its image, run it)"
docker run --rm -i \
  --privileged \
  --network "$NET" \
  --name "$ENGINE_NAME" \
  --hostname "$ENGINE_NAME" \
  --label "$POD_LABEL" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$TEMPLATE_DIR":/template:ro \
  -v "$WORK/project":/project \
  -v "$WORK/state":/state \
  -w /project \
  -e PULUMI_POD_MODE=true \
  -e PULUMI_POD_NETWORK="$NET" \
  -e PULUMI_POD_ADVERTISE_HOST="$ENGINE_NAME" \
  -e PULUMI_POD_ID="$POD_ID" \
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  --entrypoint sh \
  "$ENGINE_IMAGE" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi new /template --name '"$PROJECT_NAME"' --description "scaffolded oci nodejs program" \
      --stack '"$STACK"' --yes --force
    echo "--- scaffolded files ---"; ls -1
    pulumi up --yes --skip-preview --stack '"$STACK"'
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack '"$STACK"')"
  ' \
  2>&1 | tee "$WORK/run.log"

echo "==> asserting pulumi new scaffolded the template (Dockerfile + explicit bootstrap landed)"
# The scaffold lands in the mounted workspace, so assert on the filesystem directly.
for f in Pulumi.yaml index.js Dockerfile oci-bootstrap.sh package.json; do
  if [ ! -f "$WORK/project/$f" ]; then
    echo "!! scaffolded project is missing $f"
    exit 1
  fi
done
# The bootstrap is the point: it is a real file in the project, not hidden in a base image.
if ! grep -q 'oci-node-bootstrap' "$WORK/project/oci-bootstrap.sh"; then
  echo "!! the scaffolded oci-bootstrap.sh is not the OCI program bootstrap"
  exit 1
fi
echo "    scaffold has the user-owned Dockerfile and the explicit oci-bootstrap.sh entrypoint"

echo "==> asserting the OCI host built the program image from the scaffolded Dockerfile"
if ! grep -q "oci: building program image in builder docker:cli" "$WORK/run.log"; then
  echo "!! the program image was not built from the scaffolded build spec"
  exit 1
fi

echo "==> asserting the scaffolded program ran and returned its output"
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/run.log" | head -1)"
if [ "$GREETING" != "$EXPECTED" ]; then
  echo "!! unexpected program output: '${GREETING:-<empty>}' (wanted '$EXPECTED')"
  exit 1
fi
echo "    program output = $GREETING"
echo "==> oci-nodejs template smoke test PASS — pulumi new scaffolded a project with an explicit"
echo "    bootstrap, pulumi up built its image from the user-owned Dockerfile, and the program ran"
