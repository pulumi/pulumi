#!/usr/bin/env bash
#
# To be used as a replacement for the `go` toolchain. The wrapper
# unifies building binaries normally and building them with coverage
# support using `go test -c`.

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

PKG=github.com/pulumi/pulumi/pkg/v3/...
SDK=github.com/pulumi/pulumi/sdk/v3/...
COVERPKG=$PKG,$SDK

if [ -z "$PULUMI_TEST_COVERAGE_PATH" ]; then
    go "$@"
else
    case $1 in
        build)
            shift
            go test -c -cover -coverpkg $COVERPKG "$@"
            ;;
        install)
            echo "install command is not supported, please use build"
            exit 1
            ;;
        *)
            go "$@"
            ;;
    esac
fi
