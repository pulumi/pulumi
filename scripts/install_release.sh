#!/bin/bash
# install_release.sh will download and install the current bits from the usual share location and binplace them.
# The first argument is the release name, the second is the Git commit hash to fetch and the third is the 
# target location to install them into.

set -e

RELEASENAME=${1}
GITVER=${2}
INSTALLDIR=${3}

if [ -z "${RELEASENAME}" ]; then
    echo error: missing name of release to install
    exit 1
fi
if [ -z "${GITVER}" ]; then
    echo error: missing Git commit/tag/branch argument
    exit 1
fi
if [ -z "${INSTALLDIR}" ]; then
    echo error: missing target installation directory
    exit 2
fi

# Make the directory, download the bits, and unzip/tar them in place.
RELEASE=s3://eng.pulumi.com/releases/${RELEASENAME}/${GITVER}.tgz
echo Installing ${RELEASE} to: ${PUBTARGET}
mkdir -p ${INSTALLDIR}
aws s3 cp ${RELEASE} ${INSTALLDIR}
tar -xzf ${INSTALLDIR}/${GITVER}.tgz -C ${INSTALLDIR}

# Finally, link any packages so that we can reference those packages without NPM installing them.
if [ -f ${INSTALLDIR}/packs.txt ]; then
    while read pack; do
        pushd ${INSTALLDIR}/${pack} && yarn link && popd
    done < ${INSTALLDIR}/packs.txt
fi

