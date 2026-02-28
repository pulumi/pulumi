#!/usr/bin/env bash
# Workspace status script for Bazel --stamp support.
# Produces key-value pairs consumed by x_defs in go_binary rules.

set -euo pipefail

SCRIPTDIR=$(dirname "$0")

VERSION=$("${SCRIPTDIR}/pulumi-version.sh")
PYPI_VERSION=$("${SCRIPTDIR}/pulumi-version.sh" python)
echo "STABLE_PULUMI_VERSION ${VERSION}"
echo "STABLE_PYPI_VERSION ${PYPI_VERSION}"
