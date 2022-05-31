#!/usr/bin/env bash

set -euo pipefail

GENERIC_VERSION=v$(pulumictl get version --language generic -o)
echo GENERIC_VERSION=$GENERIC_VERSION
echo PYPI_VERSION=$(pulumictl get version --language python)
echo DOTNET_VERSION=$(pulumictl get version --language dotnet)
echo GORELEASER_CURRENT_TAG=$GENERIC_VERSION
