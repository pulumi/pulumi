#!/usr/bin/env bash
#
# Mode 1 MLC smoke test — the reverse-direction (dial-back) proof, and the
# mixed-execution proof, in one run. A plain host-run Go program registers the
# greeting component remotely by its oci:// pin; the engine (a normal host process,
# PULUMI_POD_MODE=host) starts the component container, attaches at the
# host-loopback published port, and calls Construct — whereupon the component DIALS
# BACK to the monitor at the advertise host (host.docker.internal:<monitor-port>,
# the rewrite under test; the monitor itself stays bound to the host loopback) to
# register itself and a real RandomPet child. The child resolves UNPINNED, so the
# random provider takes the STOCK path: a binary auto-installed and spawned on the
# host. Containerized component + stock nested provider + host engine — the
# gradual onramp's normal condition.
#
# Discriminating proofs:
#   - the message output embeds the child pet's generated name, which can only
#     exist if Construct completed — and Construct's registrations can only reach
#     the monitor through the advertised host.docker.internal address (the
#     component is in its own netns; the monitor is loopback-bound on the host)
#   - the greeting attach line shows 127.0.0.1:<published> (host publish path)
#   - random gets NO oci lines at all — it ran as a stock host binary
#   - no registry runs: the pin's image is tagged locally, so the store check hits
#
# Usage: run-host-engine-mlc.sh
# Requires a running Docker daemon and the repo Go toolchain (+ network for the
# stock random provider's auto-install).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-host-engine-mlc"
COMPONENT_DIR="$SMOKE_DIR/component-greeter"
PKG_DIR="$SMOKE_DIR/../.."
REPO_ROOT="$SMOKE_DIR/../../.."
LANG_GO_DIR="$REPO_ROOT/sdk/go/pulumi-language-go"
NODE_SDK_DIR="$REPO_ROOT/sdk/nodejs"

# The pin ref the program carries; the built image is tagged exactly this, so the
# host engine's store check hits and nothing listens on 5005.
IMAGE_REF="localhost:5005/pulumi/pulumi-provider-greeting:v0.1.0"

POD_ID="hostmlc-$$"
POD_LABEL="com.pulumi.pod=$POD_ID"
STACK="dev"

# Plain `docker build` may be wired to a remote builder (e.g. Depot); point
# OCI_BUILDER at a local builder.
BUILDER="${OCI_BUILDER:-desktop-linux}"

WORK="$(mktemp -d)"
mkdir -p "$WORK/bin" "$WORK/project" "$WORK/state" "$WORK/pulumi-home"

cleanup() {
  local leftovers
  leftovers="$(docker ps -aq --filter "label=$POD_LABEL" 2>/dev/null || true)"
  [ -n "$leftovers" ] && docker rm -f $leftovers >/dev/null 2>&1 || true
  docker image rm -f "$IMAGE_REF" >/dev/null 2>&1 || true
  rm -rf "$COMPONENT_DIR/sdk-bin"
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run host-engine MLC test"
  exit 1
fi

echo "==> building the branch CLI + go language host for the HOST"
( cd "$PKG_DIR" && go build -o "$WORK/bin/pulumi" ./cmd/pulumi )
( cd "$LANG_GO_DIR" && go build -o "$WORK/bin/pulumi-language-go" . )

# ── stage the branch Node SDK into the component build context ────────────────
# provider.main must honor PULUMI_PLUGIN_LISTEN_ADDRESS (host mode is address mode
# by construction); the stock npm SDK predates the contract. Rebuilt every run so a
# stale bin/ can't masquerade as the change being wrong.
echo "==> building this branch's Node SDK (sdk/nodejs -> bin/)"
( cd "$NODE_SDK_DIR" && mise exec -- make build_package >/dev/null )
if ! grep -q "PULUMI_PLUGIN_LISTEN_ADDRESS" "$NODE_SDK_DIR/bin/provider/server.js"; then
  echo "!! the built Node SDK's provider server does not honor the bind contract —"
  echo "   bin/ is stale or the build failed"
  exit 1
fi
rm -rf "$COMPONENT_DIR/sdk-bin"
cp -R "$NODE_SDK_DIR/bin" "$COMPONENT_DIR/sdk-bin"

echo "==> building the greeter MLC image (branch SDK overlaid) and tagging it as the pin ref"
docker buildx build --builder "$BUILDER" --load \
  -t "$IMAGE_REF" -f "$COMPONENT_DIR/Dockerfile.host" "$COMPONENT_DIR"

cp "$PROJECT_DIR/Pulumi.yaml" "$PROJECT_DIR/main.go" "$PROJECT_DIR/go.mod" "$WORK/project/"
echo "==> resolving the program's Go deps (host toolchain — the program runs HERE)"
( cd "$WORK/project" && go mod tidy >/dev/null 2>&1 )

export PATH="$WORK/bin:$PATH"
export PULUMI_POD_MODE=host
export PULUMI_POD_ID="$POD_ID"
export PULUMI_HOME="$WORK/pulumi-home"
export PULUMI_BACKEND_URL="file://$WORK/state"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"

cd "$WORK/project"
echo "==> pulumi stack init + up (HOST engine; containerized MLC; stock nested random)"
pulumi stack init "$STACK"
pulumi up --yes --skip-preview 2>&1 | tee "$WORK/up.log"

echo "==> asserting the component resolved by its pin and attached at a host-loopback published port"
if ! grep -q "oci: provider greeting resolved by its oci:// pin" "$WORK/up.log"; then
  echo "!! the pinned component did not resolve through the container path"
  exit 1
fi
ATTACH_LINE="$(grep 'oci: provider greeting running as container' "$WORK/up.log" | head -1)"
echo "    $ATTACH_LINE"
if ! echo "$ATTACH_LINE" | grep -qE 'attaching at 127\.0\.0\.1:[0-9]+'; then
  echo "!! expected the host engine to attach the component at 127.0.0.1:<published port>"
  exit 1
fi

echo "==> asserting the nested random provider took the STOCK path (no oci lines for it)"
if grep -q 'oci: provider random' "$WORK/up.log"; then
  echo "!! random went through the container path — unpinned packages must take the stock path"
  exit 1
fi

MESSAGE="$(pulumi stack output message)"
echo "==> asserting Construct completed through the dial-back: message=$MESSAGE"
case "$MESSAGE" in
  "hello, claire"*"(pet: "*)
    : ;;
  *)
    echo "!! message does not carry the greeting + child pet name — Construct did not"
    echo "   complete against the advertised monitor address"
    exit 1 ;;
esac

echo "==> asserting no pod-labeled containers remain (component reaped; engine was never a container)"
if docker ps -a --filter "label=$POD_LABEL" --format '{{.Names}}' | grep -q .; then
  echo "!! pod-labeled containers remain after up:"
  docker ps -a --filter "label=$POD_LABEL" --format '    {{.Names}}'
  exit 1
fi

echo "==> pulumi destroy"
pulumi destroy --yes --skip-preview 2>&1 | tee "$WORK/destroy.log"
if ! grep -qE 'deleted' "$WORK/destroy.log"; then
  echo "!! destroy did not report deleting the resources"
  exit 1
fi

echo "==> host-engine MLC smoke test PASS — a containerized component's Construct dialed the"
echo "    monitor back at host.docker.internal, built a stock-provider child on the host, and"
echo "    the whole mixed graph ran under one host engine"
