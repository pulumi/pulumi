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

build_engine_image() {
  mkdir -p "$WORK/cli"

  echo "==> cross-compiling pulumi + pulumi-language-oci (linux/$GOARCH)"
  ( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
      go build -o "$WORK/cli/pulumi-cli-linux" ./cmd/pulumi )
  ( cd "$PKG_DIR" && GOWORK=off GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 \
      go build -o "$WORK/cli/pulumi-language-oci-linux" ./cmd/pulumi-language-oci )

  echo "==> building engine image $ENGINE_IMAGE"
  docker buildx build --builder "$BUILDER" --load \
    -t "$ENGINE_IMAGE" -f "$SMOKE_DIR/Dockerfile.cli" "$WORK/cli"
}
