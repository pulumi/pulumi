#!/usr/bin/env bash
#
# Smoke test for containerized (OCI) program execution — design Option A
# (engine in-process with the host CLI; the program reaches it over loopback).
#
# It runs `pulumi up` in up to two stages, to isolate failure domains:
#
#   subprocess : Run() execs the program binary directly, monitor address passed
#                through unchanged. Proves language-host discovery + the RPC
#                sequence + Run + the backend, with zero networking variables.
#   container  : Run() `docker run`s the program image with PULUMI_POD_MODE=true,
#                rewriting the monitor address to host.docker.internal. Proves the
#                container reaches the host engine.
#
# Usage: run.sh [subprocess|container|both]   (default: both)
#
# Requires `pulumi` and `pulumi-language-oci` on PATH (point BIN at them). The
# container stage also needs a running Docker daemon.
set -euo pipefail

STAGE="${1:-both}"

SMOKE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SMOKE_DIR/project"
PROGRAM_DIR="$SMOKE_DIR/program"

# Where to find the freshly-built `pulumi` and `pulumi-language-oci` binaries.
BIN="${BIN:?set BIN to the directory containing pulumi and pulumi-language-oci}"
export PATH="$BIN:$PATH"

# The container stage builds the program image with `docker build`. On machines
# where the default builder is remote (e.g. Depot), point OCI_BUILDER at a local
# builder; it defaults to Docker Desktop's "desktop-linux".
BUILDER="${OCI_BUILDER:-desktop-linux}"

# Isolate all state in a scratch dir so the repo stays clean.
WORK="$(mktemp -d)"
export PULUMI_HOME="$WORK/home"
export PULUMI_CONFIG_PASSPHRASE="smoke-test"
BACKEND="file://$WORK/state"
STACK="dev"
mkdir -p "$WORK/state"

cleanup() {
  ( cd "$PROJECT_DIR" && pulumi destroy --yes --skip-preview --stack "$STACK" >/dev/null 2>&1 ) || true
  # Built binaries and per-stack config are scratch; keep the project dir clean.
  rm -f "$PROJECT_DIR/program" "$SMOKE_DIR/program-linux" "$PROJECT_DIR"/Pulumi.*.yaml
  rm -rf "$WORK"
}
trap cleanup EXIT

init_stack() {
  cd "$PROJECT_DIR"
  pulumi login "$BACKEND" >/dev/null
  pulumi stack select --create "$STACK" >/dev/null
}

build_image() {
  # Build the program image from the Dockerfile. We pass an explicit local
  # builder because plain `docker build` may be wired to a remote builder on this
  # machine; --load places the result in the local image store.
  local tag="$1"
  docker buildx build --builder "$BUILDER" --load \
    -t "$tag" -f "$SMOKE_DIR/Dockerfile" "$SMOKE_DIR"
}

assert_output() {
  local key="$1" want="$2"
  local got
  got="$(cd "$PROJECT_DIR" && pulumi stack output "$key" --stack "$STACK")"
  echo "    output $key = $got"
  case "$got" in
    *"$want"*) ;;
    *) echo "!! expected output $key to contain '$want', got '$got'"; exit 1 ;;
  esac
}

run_subprocess_stage() {
  echo "==> [subprocess] building demo program (native)"
  ( cd "$PROGRAM_DIR" && GOWORK=off go build -o "$PROJECT_DIR/program" . )

  echo "==> [subprocess] pulumi up"
  init_stack
  pulumi up --yes --skip-preview --stack "$STACK"
  assert_output greeting "OCI runtime"
  echo "==> [subprocess] PASS"
}

run_container_stage() {
  if ! docker info >/dev/null 2>&1; then
    echo "!! docker daemon not available — skipping container stage"
    return 0
  fi
  echo "==> [container] cross-compiling demo program (linux/$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/'))"
  local goarch
  goarch="$(uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/')"
  ( cd "$PROGRAM_DIR" && GOWORK=off GOOS=linux GOARCH="$goarch" CGO_ENABLED=0 go build -o "$SMOKE_DIR/program-linux" . )

  echo "==> [container] docker build oci-smoke-demo:latest"
  build_image oci-smoke-demo:latest

  echo "==> [container] pulumi up (PULUMI_POD_MODE=true)"
  init_stack
  # Engine runs in-process on the host; the program container dials back via the
  # docker host-gateway alias, so advertise host.docker.internal explicitly.
  PULUMI_POD_MODE=true PULUMI_POD_ADVERTISE_HOST=host.docker.internal \
    pulumi up --yes --skip-preview --stack "$STACK"
  assert_output greeting "OCI runtime"
  echo "==> [container] PASS"
}

case "$STAGE" in
  subprocess) run_subprocess_stage ;;
  container)  run_container_stage ;;
  both)       run_subprocess_stage; run_container_stage ;;
  *) echo "unknown stage '$STAGE' (want: subprocess|container|both)"; exit 2 ;;
esac

echo "==> smoke test complete"
