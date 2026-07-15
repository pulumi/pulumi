#!/usr/bin/env bash
#
# Shared dev-harness helper for the OCI smoke tests. build_engine_image
# cross-compiles THIS branch's pulumi CLI + pulumi-language-oci for linux and
# bakes them into the engine image (Dockerfile.cli). Every run-pod-*.sh duplicated
# this block verbatim; they now source this file and call the function instead.
#
# Callers must already have defined: PKG_DIR, GOARCH, WORK, BUILDER, SMOKE_DIR,
# ENGINE_IMAGE. This is test scaffolding that exercises the branch's uncommitted
# CLI — not product code, so it carries no copyright header.

# The dev-language hosts the OCI host delegates SDK generation to, as
# <module-dir>:<binary-name>. Only their *codegen* is used here (GeneratePackage is
# in-process: it binds the schema and writes files, never shelling out to go/node/
# python), so the binary alone is enough and the engine image stays toolchain-free.
# They are built from this branch rather than fetched from a release, so the codegen
# and the SDK it generates cannot skew apart.
DELEGATE_HOSTS="
sdk/go/pulumi-language-go:pulumi-language-go
sdk/nodejs/cmd/pulumi-language-nodejs:pulumi-language-nodejs
sdk/python/cmd/pulumi-language-python:pulumi-language-python
"

build_engine_image() {
  mkdir -p "$WORK/cli"
  local root
  root="$(cd "$PKG_DIR/.." && pwd)"

  echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
  ( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
      go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
  ( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
      go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

  # The proxy too, so this image matches the published one: ONE image serving both the
  # engine (default entrypoint) and the registry proxy (--entrypoint registry-proxy). The
  # wrapper runs the proxy from whatever PULUMI_POD_ENGINE_IMAGE names, so without this a
  # local engine image silently loses the proxy — and every local component then tags bare
  # instead of registry-qualified, diverging from a published run.
  echo "==> cross-compiling registry-proxy (linux/$GOARCH)"
  ( cd "$SMOKE_DIR/registry-proxy" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
      go build -o "$WORK/cli/registry-proxy-linux" . )

  # Each delegate host is its own Go module, so build it from its own directory.
  for entry in $DELEGATE_HOSTS; do
    local dir="${entry%%:*}" bin="${entry##*:}"
    echo "==> cross-compiling $bin (linux/$GOARCH)"
    ( cd "$root/$dir" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build -o "$WORK/cli/$bin-linux" . )
  done

  echo "==> building engine image $ENGINE_IMAGE"
  docker buildx build --builder "$BUILDER" --load \
    -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"
}
