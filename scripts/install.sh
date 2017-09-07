#!/bin/bash
# install.sh will download and install the current bits from the usual share location and binplace them.
# The first argument is the Git commit hash to fetch and the second is the target location to install them into.

set -e

./install_release.sh pulumi-fabric ${1} ${2}

