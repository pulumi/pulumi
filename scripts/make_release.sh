#!/bin/bash
# make_release.sh will create a build package ready for publishing.
set -o nounset -o errexit -o pipefail

ROOT=$(dirname $0)/..
PUBDIR=$(mktemp -du)
GITHASH=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITHASH}.tgz
VERSION=$(git describe --tags 2>/dev/null)

# Figure out which branch we're on. Prefer $TRAVIS_BRANCH, if set, since
# Travis leaves us at detached HEAD and `git rev-parse` just returns "HEAD".
BRANCH=${TRAVIS_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}
declare -a PUBTARGETS=(${GITHASH} ${VERSION} ${BRANCH})

# usage: run_go_build <path-to-package-to-build>
function run_go_build() {
    local bin_suffix=""
    local output_name=$(basename $(cd "$1" ; pwd))
    if [ "$(go env GOOS)" = "windows" ]; then
        bin_suffix=".exe"
    fi

    mkdir -p "${PUBDIR}/bin"
    go build -ldflags "-X main.version=${VERSION}" -o "${PUBDIR}/bin/${output_name}${bin_suffix}" "$1"
}

# usage: copy_package <path-to-module> <module-name>
copy_package() {
    local MODULE_ROOT=${PUBDIR}/node_modules/${2}

    mkdir -p "${MODULE_ROOT}"   
    cp -R "${1}" "${MODULE_ROOT}/"
    if [ -e "${MODULE_ROOT}/node_modules" ]; then
        rm -rf "${MODULE_ROOT}/node_modules"
    fi
    if [ -e "${MODULE_ROOT}/tests" ]; then
        rm -rf "${MODULE_ROOT}/tests"
    fi
}


# Build binaries
run_go_build "${ROOT}"

# Copy over the langhost and dynamic provider
cp ${ROOT}/sdk/nodejs/pulumi-langhost-nodejs ${PUBDIR}/bin/
cp ${ROOT}/sdk/nodejs/pulumi-provider-pulumi-nodejs ${PUBDIR}/bin/

# Copy packages
copy_package "${ROOT}/sdk/nodejs/bin/." "pulumi"

# Tar up the file and then print it out for use by the caller or script.
tar -czf ${PUBFILE} -C ${PUBDIR} .
echo ${PUBFILE} ${PUBTARGETS[@]}
