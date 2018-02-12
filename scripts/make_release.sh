#!/bin/bash
# make_release.sh will create a build package ready for publishing.
set -o nounset -o errexit -o pipefail

ROOT=$(dirname $0)/..
PUBDIR=$(mktemp -du)
GITHASH=$(git rev-parse HEAD)
PUBFILE=$(dirname ${PUBDIR})/${GITHASH}.tgz
VERSION=$(git describe --tags --dirty 2>/dev/null)

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
    go build \
       -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" \
       -o "${PUBDIR}/bin/${output_name}${bin_suffix}" \
       "$1"
}

# usage: copy_package <path-to-module> <module-name>
copy_package() {
    local module_root=${PUBDIR}/node_modules/$2

    mkdir -p "${module_root}"
    cp -R "$1" "${module_root}/"
    if [ -e "${module_root}/node_modules" ]; then
        rm -rf "${module_root}/node_modules"
    fi
    if [ -e "${module_root}/tests" ]; then
        rm -rf "${module_root}/tests"
    fi
}


# Build binaries
run_go_build "${ROOT}"

# Copy over the langhost and dynamic provider
cp ${ROOT}/sdk/nodejs/pulumi-langhost-nodejs-exec ${PUBDIR}/bin/
cp ${ROOT}/sdk/nodejs/pulumi-provider-pulumi-nodejs ${PUBDIR}/bin/

# Copy packages
copy_package "${ROOT}/sdk/nodejs/bin/." "@pulumi/pulumi"

# Tar up the file and then print it out for use by the caller or script.
tar -czf ${PUBFILE} -C ${PUBDIR} .
echo ${PUBFILE} ${PUBTARGETS[@]}
