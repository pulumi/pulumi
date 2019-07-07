#!/bin/bash
# make_release.sh will create a build package ready for publishing.
set -o nounset
set -o errexit
set -o pipefail

readonly ROOT=$(dirname "${0}")/..
readonly PUBDIR=$(mktemp -d)
readonly GITHASH=$(git rev-parse HEAD)
readonly PUBFILE=$(dirname "${PUBDIR})/${GITHASH}.tgz")
readonly VERSION=$("${ROOT}/scripts/get-version")

# Figure out which branch we're on. Prefer $TRAVIS_BRANCH, if set, since
# Travis leaves us at detached HEAD and `git rev-parse` just returns "HEAD".
readonly BRANCH=${TRAVIS_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}
declare -a PUBTARGETS=(${GITHASH} ${VERSION} ${BRANCH})

# usage: run_go_build <path-to-package-to-build>
run_go_build() {
    local bin_suffix=""
    local -r output_name=$(basename $(cd "${1}" ; pwd))
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
    local -r module_root="${PUBDIR}/node_modules/${2}"

    mkdir -p "${module_root}"
    cp -r "${1}" "${module_root}/"
    if [[ -e "${module_root}/node_modules" ]]; then
        rm -rf "${module_root}/node_modules"
    fi
    if [[ -e "${module_root}/tests" ]]; then
        rm -rf "${module_root}/tests"
    fi
}


# Build binaries
run_go_build "${ROOT}"
run_go_build "${ROOT}/sdk/nodejs/cmd/pulumi-language-nodejs"
run_go_build "${ROOT}/sdk/python/cmd/pulumi-language-python"
run_go_build "${ROOT}/sdk/go/pulumi-language-go"

# Copy over the language and dynamic resource providers.
cp "${ROOT}/sdk/nodejs/dist/pulumi-resource-pulumi-nodejs" "${PUBDIR}/bin/"
cp "${ROOT}/sdk/python/dist/pulumi-resource-pulumi-python" "${PUBDIR}/bin/"
cp "${ROOT}/sdk/python/cmd/pulumi-language-python-exec" "${PUBDIR}/bin/"

# Copy packages
copy_package "${ROOT}/sdk/nodejs/bin/." "@pulumi/pulumi"

# Tar up the file and then print it out for use by the caller or script.
tar -czf "${PUBFILE}" -C ${PUBDIR} .
echo ${PUBFILE} ${PUBTARGETS[@]}

exit 0
