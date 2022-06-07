#!/usr/bin/env bash
#
# To be used with Goreleaser as `gobinary` implementation as a
# replacement for the `go` toolchain.
#
# Function: coverage-enabled builds for Pulumi CLI
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

        case "$MODE" in
            normal)
                go "$@"
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
