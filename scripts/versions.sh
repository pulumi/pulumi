#!/usr/bin/env bash

set -euo pipefail

SCRIPTDIR=$(dirname "$0")
PULUMI_VERSION=$("${SCRIPTDIR}/pulumi-version.sh")
echo GENERIC_VERSION="${PULUMI_VERSION}"
echo VERSION="${PULUMI_VERSION}"
echo PYPI_VERSION="$("${SCRIPTDIR}/pulumi-version.sh" python)"
echo DOTNET_VERSION="$("${SCRIPTDIR}/pulumi-version.sh" dotnet)"
echo GORELEASER_CURRENT_TAG="v${PULUMI_VERSION}"
