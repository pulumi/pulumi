#!/usr/bin/env bash
#
# To be used with Goreleaser as `gobinary` implementation as a
# replacement for the `go` toolchain.
#
# First function: prebuilt binaries
#
# The wrapper detects and returns prebuilt binaries to skip actual go
# builds. If a binary `-o goreleaser/../some-binary` is requested but
# `goreleaer-prebuilt/../some-binary` already exists, the prebuilt
# binary is copied instead of building.
#
# Second function: coverage-enabled builds for Pulumi CLI
#
# This builds binaries via `go test -c` workaround. Disabled for
# Windows builds. Only enabled on the Pulumi CLI binaries.

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

PKG=github.com/pulumi/pulumi/pkg/v3/...
SDK=github.com/pulumi/pulumi/sdk/v3/...
COVERPKG="$PKG,$SDK"

case "$1" in
    build)
        ARGS=( "$@" )
        BUILDDIR=${ARGS[${#ARGS[@]}-1]}
        OUTPUT=${ARGS[${#ARGS[@]}-2]}

        PREBUILT="${OUTPUT/goreleaser/goreleaser-prebuilt}"

        # Since at least goreleaser 1.8.3 binaries are named
        # amd64_v1/... but prebuilt to `amd64/...`.
        PREBUILT="${PREBUILT/amd64_v1/amd64}"

        MODE=coverage

        if [ -z "$PULUMI_TEST_COVERAGE_PATH" ]; then
            MODE=normal
        fi

        # TODO: coverage-enabled builds of pulumi CLI on Windows fail
        # to parse CLI arguments correctly as in `pulumi version`,
        # disabling coverage-enabled builds on Windows.
        if [[ "$OUTPUT" == *"windows"* ]]; then
            MODE=normal
        fi

        # TODO[pulumi/pulumi#8615] - coverage-aware builds of the
        # language providers break and are disabled.
        if [[ "$BUILDDIR" != "./cmd/pulumi" ]]; then
            MODE=normal
        fi

        if [ -f "$PREBUILT" ]; then
            MODE=prebuilt
        fi

        case "$MODE" in
            normal)
                go "$@"
                ;;
            prebuilt)
                mkdir -p $(dirname "$OUTPUT")
                cp "$PREBUILT" "$OUTPUT"
                ;;
            coverage)
                shift
                go test -c -cover -coverpkg "$COVERPKG" "$@"
                ;;
        esac
        ;;
    install)
        echo "install command is not supported, please use build"
        exit 1
        ;;
    *)
        go "$@"
        ;;
esac
