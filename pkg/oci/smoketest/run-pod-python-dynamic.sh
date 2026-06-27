#!/usr/bin/env bash
#
# Python dynamic-provider smoke test — the language-agnostic counterpart to
# run-pod-dynamic.sh (Node). Proves the container host's dynamic-provider third
# branch is not Node-specific: a Python pulumi.dynamic.Resource whose CRUD code is
# serialized from the program (via dill) runs in a provider container started FROM
# THE PROGRAM IMAGE, its bootstrap shim booting `python -m pulumi.dynamic` when the
# engine sets PULUMI_OCI_ROLE=dynamic-provider. Nothing is injected.
#
# Discriminating proof: the provider's `create` reads /program-marker, a file baked
# into the PROGRAM image and present in no other, and returns it as the resource's
# output. From any other image the read would throw and `up` would fail. So stack
# output == the baked marker proves both "the dynamic resource was created" and "its
# provider ran welded to the program image".
#
# The Python program image needs the SDK's program-exec shim
# (pulumi-language-python-exec) to launch the program; that ships with the CLI, not
# the pip package, so we copy it from the repo into the build context.
#
# Usage: run-pod-python-dynamic.sh
# Requires a running Docker daemon and the repo Go toolchain (to cross-compile).
set -euo pipefail

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project-python-dynamic"
PROGRAM_DIR="$SMOKE_DIR/program-python-dynamic"
PKG_DIR="$SMOKE_DIR/../.."           # the pkg/ Go module, where the CLI + host live
REPO_ROOT="$SMOKE_DIR/../../.."      # repo root, for the python exec shim under sdk/
EXEC_SHIM="$REPO_ROOT/sdk/python/cmd/pulumi-language-python-exec"

BUILDER="${OCI_BUILDER:-desktop-linux}"
GOARCH="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"

WRAPPER="$SMOKE_DIR/pulumi-pod"
ENGINE_IMAGE="pulumi-cli-oci:latest"
PROGRAM_IMAGE="oci-smoke-python-dynamic:latest"
STACK="dev"
EXPECTED_MARKER="oci-dynamic-welded-to-python-image"

WORK="$(mktemp -d)"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
mkdir -p "$WORK/cli" "$WORK/ctx" "$WORK/project"

cleanup() {
  # The wrapper reclaims each pod; this only clears the cross-compiled binaries and
  # the scratch dir.
  rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "!! docker daemon not available — cannot run Python dynamic-provider test"
  exit 1
fi

echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

echo "==> building engine image $ENGINE_IMAGE"
docker buildx build --builder "$BUILDER" --load \
  -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"

echo "==> assembling Python program build context (program + the SDK program-exec shim)"
cp "$PROGRAM_DIR"/* "$WORK/ctx/"
cp "$EXEC_SHIM" "$WORK/ctx/"

echo "==> building Python program image $PROGRAM_IMAGE (pip install pulumi, bakes /program-marker)"
docker buildx build --builder "$BUILDER" --load \
  -t "$PROGRAM_IMAGE" -f "$PROGRAM_DIR/Dockerfile" "$WORK/ctx"

cp "$PROJECT_DIR/Pulumi.yaml" "$WORK/project/"

export PULUMI_POD_ENGINE_IMAGE="$ENGINE_IMAGE"
export PULUMI_POD_MOUNT_DIR="$WORK/project"
export PULUMI_POD_PROGRAM_IMAGE="$PROGRAM_IMAGE"

echo "==> pulumi-pod: stack init + up + output (engine runs the Python dynamic provider FROM the program image)"
"$WRAPPER" stack init "$STACK"
"$WRAPPER" up --yes --skip-preview 2>&1 | tee "$WORK/up.log"
MARKER="$("$WRAPPER" stack output marker)"

echo "==> asserting the Python dynamic provider ran from the program image and produced the baked marker"
if ! grep -q 'oci: provider pulumi-python is a dynamic provider' "$WORK/up.log"; then
  echo "!! the engine did not run the Python dynamic provider from the program image"
  exit 1
fi
if [ "$MARKER" != "$EXPECTED_MARKER" ]; then
  echo "!! marker mismatch: got '${MARKER:-<empty>}', want '$EXPECTED_MARKER'"
  echo "   (the Python dynamic provider did not run welded to the program image)"
  exit 1
fi
echo "    marker = $MARKER"
echo "==> Python dynamic-provider smoke test PASS — dynamic-provider execution is language-agnostic"
