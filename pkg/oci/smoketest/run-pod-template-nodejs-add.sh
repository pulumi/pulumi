#!/usr/bin/env bash
#
# `pulumi package add` into an oci-nodejs scaffold — proves the BUILD-OWNED, DECLARE-ONLY
# link for nodejs, with real codegen. The package-add go test proves link *runs* in its own
# image; this proves the *nodejs* link command (npm pkg set, declare-only) produces a valid
# package.json edit against a real generated SDK, integrated with the template scaffold.
#
# Flow:
#   1. build the engine image + bake in pulumi-language-nodejs (the codegen delegate)
#   2. start the registry-proxy (synthesizes the random provider image on pull)
#   3. `pulumi new oci-nodejs` to scaffold a project
#   4. `pulumi package add oci://<proxy>/pulumi-provider-random:vX` — the OCI host delegates
#      SDK generation to pulumi-language-nodejs (a TS SDK lands in sdks/random), then runs the
#      template's DECLARE-ONLY link command (npm pkg set a file: dep) in link.image (node)
#   5. assert the SDK was generated AND package.json now declares it as a file: dependency
#
# This is the link half (declare). Materialize (the program image's `npm install`) + use it
# at runtime re-exercises the proven program-build path; left as a follow-up.
#
# Usage: run-pod-template-nodejs-add.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
TEMPLATE_DIR="$SMOKE_DIR/templates/oci-nodejs"
PROXY_DIR="$SMOKE_DIR/registry-proxy"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
# pulumi-language-nodejs is its own Go module.
LANG_NODEJS_DIR="$REPO_ROOT/sdk/nodejs/cmd/pulumi-language-nodejs"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

PROVIDER_PKG="random"
PROVIDER_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
ENGINE_IMAGE_CODEGEN="pulumi-cli-oci-nodejs:latest"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
PROJECT_NAME="oci-add-smoke"

REGISTRY_PORT=5005
REGISTRY_HOST="localhost:$REGISTRY_PORT"
PROXY_NAME="$NET-registry"
IMAGE_REF="$REGISTRY_HOST/pulumi-provider-$PROVIDER_PKG:v$PROVIDER_VERSION"
OCI_SOURCE="oci://$IMAGE_REF"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/project" "$WORK/state"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker rm -f "$PROXY_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  docker image rm -f "$IMAGE_REF" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run template package-add test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling delegate language host pulumi-language-nodejs (linux/$GOARCH)"
( cd "$LANG_NODEJS_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-nodejs-linux" . )

echo "==> building codegen engine image $ENGINE_IMAGE_CODEGEN (base + pulumi-language-nodejs)"
cat > "$WORK/cli/Dockerfile.codegen" <<EOF
FROM $ENGINE_IMAGE
COPY pulumi-language-nodejs-linux /usr/local/bin/pulumi-language-nodejs
EOF
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE_CODEGEN" -f "$WORK/cli/Dockerfile.codegen" "$WORK/cli"

echo "==> cross-compiling the registry-proxy (linux/$GOARCH)"
( cd "$PROXY_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/registry-proxy-linux" . )

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null

echo "==> starting registry-proxy $PROXY_NAME on $REGISTRY_HOST"
docker run -d --name "$PROXY_NAME" --label "$POD_LABEL" -p "$REGISTRY_PORT:5000" \
  -e PROXY_TARGET_ARCH="$GOARCH" \
  -v "$WORK/registry-proxy-linux":/registry-proxy:ro \
  alpine sh -c 'apk add --no-cache ca-certificates >/dev/null 2>&1 && exec /registry-proxy' >/dev/null
for _ in $(seq 1 30); do
  curl -sf "http://$REGISTRY_HOST/v2/" >/dev/null 2>&1 && break
  sleep 0.5
done
if ! curl -sf "http://$REGISTRY_HOST/v2/" >/dev/null 2>&1; then
  echo "!! registry-proxy did not come up on $REGISTRY_HOST"
  docker logs "$PROXY_NAME" 2>&1 | tail -20
  exit 1
fi

echo "==> pulumi new oci-nodejs + package add $OCI_SOURCE (generate SDK + declare-only link)"
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
  -e PULUMI_POD_PLUGIN_REGISTRY="$REGISTRY_HOST" \
  -e PULUMI_BACKEND_URL=file:///state \
  -e PULUMI_CONFIG_PASSPHRASE="$PULUMI_CONFIG_PASSPHRASE" \
  --entrypoint sh \
  "$ENGINE_IMAGE_CODEGEN" \
  -c '
    set -e
    pulumi login "$PULUMI_BACKEND_URL"
    pulumi new /template --name '"$PROJECT_NAME"' --description "oci nodejs add test" \
      --stack '"$STACK"' --yes --force
    pulumi package add '"$OCI_SOURCE"'
    echo "--- package.json after add ---"; cat package.json
  ' \
  2>&1 | tee "$WORK/run.log"

echo "==> asserting the nodejs SDK was generated (codegen delegated to pulumi-language-nodejs)"
if ! find "$WORK/project/sdks" -name '*.ts' 2>/dev/null | grep -q .; then
  echo "!! no TypeScript SDK files were generated under sdks/ — nodejs codegen delegation failed"
  find "$WORK/project" -maxdepth 3 -type f 2>/dev/null | sed 's/^/    /' | head -40
  exit 1
fi
echo "    nodejs SDK generated: $(find "$WORK/project/sdks" -name '*.ts' | head -1 | sed "s#$WORK/project/##")"

echo "==> asserting the DECLARE-ONLY link wrote a file: dependency into package.json"
# npm may normalize the path (file:sdks/... or file:./sdks/...); match either.
if ! grep -qE 'file:\.?/?sdks/' "$WORK/project/package.json"; then
  echo "!! package.json has no file: dependency on the generated SDK — the declare-only link did not run"
  echo "--- package.json ---"; cat "$WORK/project/package.json"
  exit 1
fi
echo "    package.json declares the SDK: $(grep -oE '"[^"]*": *"file:[^"]*sdks/[^"]*"' "$WORK/project/package.json")"

echo "==> asserting the oci:// ref (not a path) was recorded in Pulumi.yaml"
if ! grep -q "$OCI_SOURCE" "$WORK/project/Pulumi.yaml"; then
  echo "!! the oci:// ref was not recorded in Pulumi.yaml"
  exit 1
fi
echo "==> oci-nodejs package-add smoke test PASS — package add generated a nodejs SDK and the"
echo "    declare-only link command wrote a file: dependency (declare); the build materializes it"
