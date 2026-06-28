#!/usr/bin/env bash
#
# `pulumi new oci-python` template smoke test — scaffold, build, run. Python is the
# template with the bootstrap wrinkle: like Node it needs an explicit oci-bootstrap.sh
# entrypoint (the SDK reads the monitor address via the program-exec shim, not from env
# like Go), and UNLIKE Node, its run harness (pulumi-language-python-exec) ships with the
# CLI, not the pip package — so the template VENDORS it (see the Dockerfile note). This
# proves the explicit-bootstrap scaffold works for Python end to end.
#
# Usage: run-pod-template-python.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile the CLI).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
TEMPLATE_DIR="$SMOKE_DIR/templates/oci-python"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
PROJECT_NAME="oci-python-smoke"
EXPECTED="hello from $PROJECT_NAME, an OCI python program"

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

echo "==> pulumi new oci-python -> up -> output (scaffold a Python project, build its image, run it)"
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
    pulumi new /template --name '"$PROJECT_NAME"' --description "scaffolded oci python program" \
      --stack '"$STACK"' --yes --force
    pulumi up --yes --skip-preview --stack '"$STACK"'
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack '"$STACK"')"
  ' \
  2>&1 | tee "$WORK/run.log"

echo "==> asserting pulumi new scaffolded the Python template (explicit bootstrap + vendored shim)"
for f in Pulumi.yaml __main__.py requirements.txt Dockerfile oci-bootstrap.sh pulumi-language-python-exec; do
  if [ ! -f "$WORK/project/$f" ]; then
    echo "!! scaffolded project is missing $f"
    exit 1
  fi
done
echo "    scaffold has the explicit oci-bootstrap.sh and the vendored pulumi-language-python-exec"

echo "==> asserting the program image was built from the scaffolded Dockerfile"
if ! grep -q "oci: building program image in builder docker:cli" "$WORK/run.log"; then
  echo "!! the program image was not built from the scaffolded build spec"
  exit 1
fi

echo "==> asserting the scaffolded program ran (via the bootstrap + exec shim) and returned its output"
GREETING="$(sed -n 's/.*SMOKE greeting=<<\(.*\)>>.*/\1/p' "$WORK/run.log" | head -1)"
if [ "$GREETING" != "$EXPECTED" ]; then
  echo "!! unexpected program output: '${GREETING:-<empty>}' (wanted '$EXPECTED')"
  exit 1
fi
echo "    program output = $GREETING"
echo "==> oci-python template smoke test PASS — pulumi new scaffolded a Python project with an"
echo "    explicit bootstrap + vendored run harness, pulumi up built and ran it"
