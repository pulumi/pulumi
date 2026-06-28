#!/usr/bin/env bash
#
# `pulumi new oci-go` template smoke test — scaffold, build, run. Go is the trivial
# template: a Go Pulumi program needs NO bootstrap shim (the Go SDK reads the
# monitor/engine address from the PULUMI_* environment), so the program image's entrypoint
# is just the compiled binary. This proves the no-shim Go scaffold end to end: `pulumi new`
# produces a project whose Dockerfile compiles the binary, and `pulumi up` builds it and
# runs it as a pod container, returning the program's exported output.
#
# Usage: run-pod-template-go.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile the CLI).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
TEMPLATE_DIR="$SMOKE_DIR/templates/oci-go"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
PROJECT_NAME="oci-go-smoke"
EXPECTED="hello from $PROJECT_NAME, an OCI go program"

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

echo "==> pulumi new oci-go -> up -> output (scaffold a Go project, compile its image, run it)"
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
    pulumi new /template --name '"$PROJECT_NAME"' --description "scaffolded oci go program" \
      --stack '"$STACK"' --yes --force
    pulumi up --yes --skip-preview --stack '"$STACK"'
    printf "SMOKE greeting=<<%s>>\n" "$(pulumi stack output greeting --stack '"$STACK"')"
  ' \
  2>&1 | tee "$WORK/run.log"

echo "==> asserting pulumi new scaffolded the Go template (no bootstrap shim — Go reads env)"
for f in Pulumi.yaml main.go go.mod Dockerfile; do
  if [ ! -f "$WORK/project/$f" ]; then
    echo "!! scaffolded project is missing $f"
    exit 1
  fi
done
if [ -f "$WORK/project/oci-bootstrap.sh" ]; then
  echo "!! a Go program should not need a bootstrap shim, but one was scaffolded"
  exit 1
fi
echo "    scaffold is a plain Go project (main.go + Dockerfile, no shim)"

echo "==> asserting the program image was built from the scaffolded Dockerfile"
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
echo "==> oci-go template smoke test PASS — pulumi new scaffolded a Go project (no shim), pulumi up"
echo "    compiled its image from the user-owned Dockerfile, and the program ran"
