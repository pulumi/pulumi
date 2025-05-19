#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
#shellcheck source=utils.sh
source "${SCRIPT_ROOT}/utils.sh"

ensureSet "${NODE_DISTRO}" "NODE_DISTRO" || exit 1
ensureSet "${NODE_VERSION}" "NODE_VERSION" || exit 1
ensureSet "${YARN_VERSION}" "YARN_VERSION" || exit 1

export DEBIAN_FRONTEND=noninteractive

NODE_REPO="node_${NODE_VERSION}"

curl -sSL https://deb.nodesource.com/gpgkey/nodesource.gpg.key | apt-key add -
echo "deb https://deb.nodesource.com/${NODE_REPO} ${NODE_DISTRO} main" > /etc/apt/sources.list.d/nodesource.list
echo "deb-src https://deb.nodesource.com/${NODE_REPO} ${NODE_DISTRO} main" >> /etc/apt/sources.list.d/nodesource.list

apt-get update
apt-get install -y nodejs

corepack enable
yarn set version "${YARN_VERSION}"
