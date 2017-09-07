#!/bin/bash
# make_release.sh will create a build package ready for publishing.
set -e

PUBDIR=$(mktemp -du)
GITVER=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITVER}.tgz
PUBPREFIX=s3://eng.pulumi.com/releases/pulumi-fabric
declare -a PUBTARGETS=(${GITVER} $(git describe --tags) $(git rev-parse --abbrev-ref HEAD))

ROOT=$(dirname $0)/..

# Copy the binaries and packs.
mkdir -p ${PUBDIR}/bin/
cp ${GOPATH}/bin/lumi ${PUBDIR}/bin/
mkdir -p ${PUBDIR}/sdk/nodejs/
cp ${ROOT}/sdk/nodejs/pulumi-langhost-nodejs ${PUBDIR}/sdk/nodejs/
cp -R ${ROOT}/sdk/nodejs/package.json ${PUBDIR}/sdk/nodejs/package.json
cp -R ${ROOT}/sdk/nodejs/bin/. ${PUBDIR}/sdk/nodejs/bin/
cp -R ${ROOT}/sdk/nodejs/node_modules/. ${PUBDIR}/sdk/nodejs/node_modules/
echo sdk/nodejs/ >> ${PUBDIR}/packs.txt

# Tar up the file and then print it out for use by the caller or script.
tar -czf ${PUBFILE} -C ${PUBDIR} .
echo ${PUBFILE} ${PUBTARGETS[@]}

