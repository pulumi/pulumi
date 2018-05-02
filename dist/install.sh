#!/bin/bash

set -o nounset -o errexit -o pipefail
SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

INSTALL_LOG=${SCRIPT_DIR}/install.log
PULUMI_INSTALL_PATH=${PULUMI_INSTALL_PATH:-/usr/local/pulumi}

function on_exit() {
    if [ "$?" -ne 0 ]; then
        >&2 echo "error: There was a problem during installation.  Please join our #community-discussion"
        >&2 echo "    Slack channel (https://pulumi.slack.com/app_redirect?channel=community-discussion) for help."
        >&2 echo "    We may need the install log file which can be found at:"
        >&2 echo "        ${INSTALL_LOG}"
    fi
}

if [ $(id -u) -eq 0 ]; then
    >&2 echo "error: install.sh should not be run as root, please run it as a regular user."
    exit 1
fi

trap on_exit EXIT

[ ! -e "${INSTALL_LOG}" ] || rm "${INSTALL_LOG}"

echo "Installing Pulumi to ${PULUMI_INSTALL_PATH}" | tee -a "${INSTALL_LOG}"

if [ ! -w $(dirname "${PULUMI_INSTALL_PATH}") ]; then
    echo "Creating ${PULUMI_INSTALL_PATH} as root" >> "${INSTALL_LOG}"

    echo "It looks like you don't have write access to /usr/local, this script will sudo"
    echo "to create ${PULUMI_INSTALL_PATH}, so you may be prompted for your password."

    sudo sh -c "[ ! -e \"${PULUMI_INSTALL_PATH}\" ] || rm -rf \"${PULUMI_INSTALL_PATH}\" ; mkdir \"${PULUMI_INSTALL_PATH}\" ; chown -R $(id -u):$(id -g) \"${PULUMI_INSTALL_PATH}\""
else
    echo "Creating ${PULUMI_INSTALL_PATH} as regular user" >> "${INSTALL_LOG}"

    [ ! -e "${PULUMI_INSTALL_PATH}" ] || rm -rf "${PULUMI_INSTALL_PATH}"
    mkdir "${PULUMI_INSTALL_PATH}"
fi

echo "Copying bin to ${PULUMI_INSTALL_PATH}" >> "${INSTALL_LOG}"
cp -r "${SCRIPT_DIR}/bin" "${PULUMI_INSTALL_PATH}/"

echo "Pulumi is now installed, please add ${PULUMI_INSTALL_PATH}/bin to your \$PATH"
