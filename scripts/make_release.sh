#!/bin/bash
# make_release.sh will create a build package ready for publishing.
set -e

ROOT=$(dirname $0)/..
PUBDIR=$(mktemp -du)
GITVER=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITVER}.tgz

# Figure out which branch we're on. Prefer $TRAVIS_BRANCH, if set, since
# Travis leaves us at detached HEAD and `git rev-parse` just returns "HEAD".
BRANCH=${TRAVIS_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}
declare -a PUBTARGETS=(${GITVER} $(git describe --tags) ${BRANCH})

# Copy the binaries, scripts, and packs.
mkdir -p ${PUBDIR}/bin/
cp ${GOPATH}/bin/pulumi ${PUBDIR}/bin/
mkdir -p ${PUBDIR}/sdk/
cp -R ${GOPATH}/src/github.com/pulumi/home/scripts/. ${PUBDIR}/scripts/
cp -R ${ROOT}/sdk/nodejs/bin/. ${PUBDIR}/sdk/nodejs/
cp -R ${ROOT}/sdk/nodejs/node_modules/. ${PUBDIR}/sdk/nodejs/node_modules/
echo sdk/nodejs/ >> ${PUBDIR}/packs.txt

# Tar up the file and then print it out for use by the caller or script.
tar -czf ${PUBFILE} -C ${PUBDIR} .
echo ${PUBFILE} ${PUBTARGETS[@]}
