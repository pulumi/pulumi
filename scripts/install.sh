#!/bin/bash
# install.sh will download and install the current bits from the usual share location and binplace them.
# The first argument is the Git commit hash to fetch and the second is the target location to install them into.

set -e

GITVER=${1}
INSTALLDIR=${2}

if [ -z "${GITVER}" ]; then
    echo error: missing Git version argument
    exit 1
fi
if [ -z "${INSTALLDIR}" ]; then
    echo error: missing target installation directory
    exit 2
fi

# Make the directory, download the bits, and unzip/tar them in place.
RELEASE=s3://eng.pulumi.com/releases/${GITVER}.tgz
echo Installing ${RELEASE} to: ${PUBTARGET}
mkdir -p ${INSTALLDIR}
aws s3 cp ${RELEASE} ${INSTALLDIR}
tar -xzf ${INSTALLDIR}/${GITVER}.tgz -C ${INSTALLDIR}


