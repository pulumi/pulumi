#!/bin/bash
# publish.sh will publish the current bits from the usual build location to an S3 build share.

set -e

PUBDIR=$(mktemp -du)
GITVER=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITVER}.tgz
PUBPREFIX=s3://eng.pulumi.com/releases/pulumi-fabric
declare -a PUBTARGETS=(${GITVER} $(git describe --tags) $(git rev-parse --abbrev-ref HEAD))

ROOT=$(dirname $0)/..

# Ensure the repo isn't dirty.
git diff-index --quiet HEAD -- || \
    test -n "${PUBFORCE}" || \
    (echo "error: Cannot publish a dirty repo; set PUBFORCE=true to override" && exit 99)

# Copy the binaries and packs.
mkdir -p ${PUBDIR}/bin/
cp ${GOPATH}/bin/lumi ${PUBDIR}/bin/
mkdir -p ${PUBDIR}/sdk/nodejs/
cp ${ROOT}/sdk/nodejs/pulumi-langhost-nodejs ${PUBDIR}/sdk/nodejs/
cp -R ${ROOT}/sdk/nodejs/package.json ${PUBDIR}/sdk/nodejs/package.json
cp -R ${ROOT}/sdk/nodejs/bin/. ${PUBDIR}/sdk/nodejs/bin/
cp -R ${ROOT}/sdk/nodejs/node_modules/. ${PUBDIR}/sdk/nodejs/node_modules/
echo sdk/nodejs/ >> ${PUBDIR}/packs.txt

# Tar up the release and upload it to our S3 bucket.
tar -czf ${PUBFILE} -C ${PUBDIR} .
for target in ${PUBTARGETS[@]}; do
    PUBTARGET=${PUBPREFIX}/${target}.tgz
    echo Publishing ${GITVER} to: ${PUBTARGET}
    if [ -z "${FIRSTTARGET}" ]; then
        # Upload the first one for real.
        aws s3 cp ${PUBFILE} ${PUBTARGET}
        FIRSTTARGET=${PUBTARGET}
    else
        # For all others, reuse the first target to avoid re-uploading.
        aws s3 cp ${FIRSTTARGET} ${PUBTARGET}
    fi
done

