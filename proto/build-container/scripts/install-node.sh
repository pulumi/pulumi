#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
#shellcheck source=utils.sh
source "${SCRIPT_ROOT}/utils.sh"

ensureSet "${NODE_VERSION}" "NODE_VERSION" || exit 1
ensureSet "${YARN_VERSION}" "YARN_VERSION" || exit 1

export DEBIAN_FRONTEND=noninteractive

NODE_REPO="node_${NODE_VERSION}"

# Use modern keyring-based GPG key installation (NodeSource new method)
mkdir -p /etc/apt/keyrings
curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/${NODE_REPO} nodistro main" > /etc/apt/sources.list.d/nodesource.list

apt-get update
apt-get install -y nodejs

corepack enable
yarn set version "${YARN_VERSION}"
