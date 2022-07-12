#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
#shellcheck source=utils.sh
source "${SCRIPT_ROOT}/utils.sh"

ensureSet "${PYTHON_VERSION}" "PYTHON_VERSION" || exit 1
ensureSet "${PIPENV_VERSION}" "PIPENV_VERSION" || exit 1
ensureSet "${AWSCLI_VERSION}" "AWSCLI_VERSION" || exit 1
ensureSet "${WHEEL_VERSION}" "WHEEL_VERSION" || exit 1
ensureSet "${TWINE_VERSION}" "TWINE_VERSION" || exit 1

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y "python${PYTHON_VERSION}" python3-pip python3-dev
update-alternatives --install /usr/bin/python3 python3 "/usr/bin/python${PYTHON_VERSION}" 1
python3 --version

# update pip and setuptools
pip3 install --upgrade pip
python3 -m pip install --upgrade setuptools

pip3 install --user "pipenv==${PIPENV_VERSION}"
pip3 install --user "wheel==${WHEEL_VERSION}" "twine==${TWINE_VERSION}"