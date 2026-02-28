#!/usr/bin/env bash
# Workspace status script for Bazel --stamp support.
# Produces key-value pairs consumed by x_defs in go_binary rules.

set -euo pipefail

SCRIPTDIR=$(dirname "$0")

VERSION=$("${SCRIPTDIR}/pulumi-version.sh")
echo "STABLE_PULUMI_VERSION ${VERSION}"
