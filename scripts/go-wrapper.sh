#!/usr/bin/env bash
#
# To be used with Goreleaser as `gobinary` implementation as a
# replacement for the `go` toolchain.
#
# Function: coverage-enabled builds for Pulumi CLI
#
# This builds binaries via `go test -c` workaround. Disabled for
# Windows builds. Only enabled on the Pulumi CLI binaries.

set -euo pipefail

PULUMI_TEST_COVERAGE_PATH=${PULUMI_TEST_COVERAGE_PATH:-}
PULUMI_BUILD_MODE=${PULUMI_BUILD_MODE:-}

COVER_PACKAGES=( \
    "github.com/pulumi/pulumi/pkg/v3/..." \
    "github.com/pulumi/pulumi/sdk/v3/..." \
    "github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3/..." \
    "github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3/..." \
)

# Join COVER_PACKAGES with commas.
COVERPKG=$(IFS=,; echo "${COVER_PACKAGES[*]}")

# If it's a production or local build - building for macOS on macOS - use CGO for DNS resolver functionality.
#
# See: https://github.com/golang/go/issues/12524
if [ "$(go env GOOS)" = "darwin" ] && [ "$(uname)" = "Darwin" ]; then
    # `go env GOOS` returns "darwin" when cross-compiling to macOS
    # `uname` returns "Darwin" on macOS
    export CGO_ENABLED=1
else
    export CGO_ENABLED=0
fi

case "$1" in
    build)
        MODE="$PULUMI_BUILD_MODE"
        if [ -z "$MODE" ]; then
            # If a build mode was not specified,
            # guess based on whether a coverage path was supplied.
            MODE=normal
            if [ -z "$PULUMI_TEST_COVERAGE_PATH" ]; then
                MODE=normal
            fi
        fi

        case "$MODE" in
            normal)
                go "$@"
                ;;
            coverage)
                shift
                go build -cover -coverpkg "$COVERPKG" "$@"
                ;;
            *)
                echo "unknown build mode: $MODE"
                exit 1
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
