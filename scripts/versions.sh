#!/usr/bin/env bash

set -euo pipefail

# TODO: use pulumictl or another tool to get other versions
GENERIC_VERSION="$1"
echo GENERIC_VERSION="$GENERIC_VERSION"
echo PYPI_VERSION="$GENERIC_VERSION"
echo DOTNET_VERSION="$GENERIC_VERSION"
echo GORELEASER_CURRENT_TAG="v$GENERIC_VERSION"
