#!/usr/bin/env bash
#
# MLC smoke test (design Phase 6, the prototype payoff): a Go program consumes a
# Node multi-language component, both running as containers in the pod. The Go
# program creates a greeting:index:Greeter; the engine resolves the `greeting`
# provider, and — because the container host treats a component provider like any
# other provider — starts the Node component image as a sibling container, attaches
# to it, and calls Construct. The Node code runs the component and returns its
# outputs, which flow back to the Go program as a stack output.
#
# The component does not merely round-trip a string: it registers a real
# provider-backed child (random.RandomPet). That forces the engine to lazily start
# the `random` provider as *another* pod container from inside Construct — the
# recursive provider-start that proves the whole chain (program -> MLC container ->
# child RegisterResource -> engine -> provider container -> monitor), not just the
# Construct hop. The child's generated pet name flows back out through the message.
#
# This exercises the Construct flow (engine -> provider.Construct -> component
# container) end-to-end, the last untouched surface, and demonstrates the
# program=component unification: a Go program drives a Node component, uniformly,
# as pod containers.
#
# Usage: run-pod-mlc.sh
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_DIR/lib-engine.sh" # shared dev-harness: cross-compile CLI + build engine image
PROJECT_DIR="$SMOKE_DIR/project-mlc"
PROGRAM_DIR="$SMOKE_DIR/program-mlc"
COMPONENT_DIR="$SMOKE_DIR/component-greeter"
PKG_DIR="$SMOKE_DIR/../.."

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

# The component's package + version. The consuming program pins this version, so
# the container host resolves the image by the pulumi-provider-<name>:v<version>
# convention.
COMPONENT_PKG="greeting"
COMPONENT_VERSION="0.1.0"

# The component registers a random.RandomPet child. That makes the engine lazily
# start the stock `random` provider as another pod container *during Construct* —
# the recursive provider-start this test exists to prove. Its version is pinned to
# the @pulumi/random the component's package.json depends on, so the container
# host resolves pulumi-provider-random:v<version> by the same convention.
RANDOM_PKG="random"
RANDOM_VERSION="4.21.0"

POD_ID="smoke-$$"
NET="pulumi-pod-$POD_ID"
ENGINE_NAME="$NET-engine"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-mlc:latest"
COMPONENT_IMAGE="pulumi-provider-$COMPONENT_PKG:v$COMPONENT_VERSION"
RANDOM_IMAGE="pulumi-provider-$RANDOM_PKG:v$RANDOM_VERSION"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"
EXPECTED_FRAGMENT="from a Node multi-language component"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/provctx" "$WORK/state" "$WORK/project"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -f "$SMOKE_DIR/program-linux"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run MLC test"
  exit 1
fi

build_engine_image

echo "==> cross-compiling Go consumer program (linux/$GOARCH)"
( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$SMOKE_DIR/program-linux" . )

echo "==> building Go consumer image $PROGRAM_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"

echo "==> building Node component image $COMPONENT_IMAGE (npm install @pulumi/pulumi)"
docker buildx build --builder "$BUILDER" --load \
  -t "$COMPONENT_IMAGE" -f "$COMPONENT_DIR/Dockerfile" "$COMPONENT_DIR"

echo "==> downloading stock $RANDOM_PKG provider v$RANDOM_VERSION (linux/$GOARCH) and wrapping it"
# The component's child needs this provider, but the engine resolves and starts it
# by convention — we only have to make the image present, exactly as a stateless
# provider test would. We do not build it against this branch.
RANDOM_URL="https://get.pulumi.com/releases/plugins/pulumi-resource-$RANDOM_PKG-v$RANDOM_VERSION-linux-$GOARCH.tar.gz"
curl -fsSL "$RANDOM_URL" -o "$WORK/random.tar.gz"
tar -xzf "$WORK/random.tar.gz" -C "$WORK/provctx" "pulumi-resource-$RANDOM_PKG"
mv "$WORK/provctx/pulumi-resource-$RANDOM_PKG" "$WORK/provctx/provider-bin"
docker buildx build --builder "$BUILDER" --load \
  -t "$RANDOM_IMAGE" -f "$SMOKE_DIR/Dockerfile.provider" "$WORK/provctx"

echo "==> creating pod network $NET"
docker network create "$NET" >/dev/null
cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

echo "==> running engine container $ENGINE_NAME (Go program + Node component as sibling containers)"
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
    printf "SMOKE message=<<%s>>\n" "$(pulumi stack output message --stack "$STACK")"
  ' \
  2>&1 | tee "$WORK/engine.log"

echo "==> asserting the engine started the component container and Construct returned its output"
if ! grep -q "oci: provider $COMPONENT_PKG running as container" "$WORK/engine.log"; then
  echo "!! the engine did not start the Node component as a container"
  exit 1
fi

# The payoff: the component's Construct registered a RandomPet child, which drove
# the engine to lazily start the `random` provider as *another* pod container —
# the recursive provider-start, from inside Construct. Prove both the container
# start and that the child resource was actually created.
echo "==> asserting the component recursively started the random provider and created its child"
if ! grep -q "oci: provider $RANDOM_PKG running as container" "$WORK/engine.log"; then
  echo "!! the component did not recursively start the random provider as a container"
  exit 1
fi
# Match the `created` line specifically, not `creating` — require the child's
# Create to have completed, not merely started. (The pet-name check below is the
# stronger proof, since a concrete generated name can't exist without a finished
# round-trip; this is belt-and-suspenders.)
if ! grep -qE "random:index:RandomPet .*created" "$WORK/engine.log"; then
  echo "!! the component's RandomPet child was not created"
  exit 1
fi

MESSAGE="$(sed -n 's/.*SMOKE message=<<\(.*\)>>.*/\1/p' "$WORK/engine.log" | head -1)"
# The message must carry both the component's own text and the child's generated
# pet name — proving the child's output propagated back out through Construct.
case "$MESSAGE" in
  *"$EXPECTED_FRAGMENT"*"(pet: "*) echo "    message = $MESSAGE" ;;
  *) echo "!! component output missing child pet name or unexpected: '${MESSAGE:-<empty>}'"; exit 1 ;;
esac
echo "==> MLC smoke test PASS — a Go program drove a Node component that built real"
echo "    infrastructure (a RandomPet), recursively starting the random provider container"
