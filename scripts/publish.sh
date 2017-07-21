#!/bin/bash
set -ex

LUMIROOT=/usr/local/lumi
LUMILIB=${LUMIROOT}/packs

PUBDIR=$(mktemp -du)
GITVER=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITVER}.tgz
PUBTARGET=s3://eng.pulumi.com/releases/${GITVER}.tgz

ROOT=$(dirname $0)/..

# Make sure the repo isn't dirty.
git diff-index --quiet HEAD -- || \
    test -n "${PUBFORCE}" || \
    (echo "error: Cannot publish a dirty repo; set PUBFORCE=true to override" && exit 99)

# If it isn't, or publication was forced, do it.
echo Publishing to: ${PUBTARGET}
mkdir -p ${PUBDIR}/cmd ${PUBDIR}/packs

# Copy the binaries and packs.
cp ${GOPATH}/bin/lumi ${PUBDIR}/cmd
cp ${ROOT}/cmd/lumijs/lumijs ${PUBDIR}/cmd
cp -R ${ROOT}/cmd/lumijs/bin/ ${PUBDIR}/cmd/lumijs.bin
cp -R ${ROOT}/cmd/lumijs/node_modules/ ${PUBDIR}/cmd/lumijs.bin/node_modules/
cp -R ${LUMILIB}/lumirt ${PUBDIR}/packs/lumirt
cp -R ${LUMILIB}/lumijs ${PUBDIR}/packs/lumijs
cp -R ${LUMILIB}/lumi ${PUBDIR}/packs/lumi

# Fix up the LumiJS script so that it can run in place.
sed -i.bak 's/"\.\/bin\/cmd"/"\.\/lumijs.bin\/cmd"/g' ${PUBDIR}/cmd/lumijs
rm ${PUBDIR}/cmd/lumijs.bak

# Tar up the release and upload it to our S3 bucket.
tar -czf ${PUBFILE} -C ${PUBDIR} .
aws s3 cp ${PUBFILE} ${PUBTARGET}

