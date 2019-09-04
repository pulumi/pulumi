#!/bin/bash
# publish_packages.sh uploads our packages to package repositories like npm
set -o nounset
set -o errexit
set -o pipefail
readonly ROOT=$(dirname "${0}")/..

NPM_VERSION=$("${ROOT}/scripts/get-version")
"${ROOT}/scripts/build-sdk.sh" $(echo ${NPM_VERSION} | sed -e 's/\+.*//g') $(git rev-parse HEAD)

if [[ "${TRAVIS_PUBLISH_PACKAGES:-}" == "true" ]]; then
    echo "Publishing NPM package to NPMjs.com:"
    NPM_TAG="dev"

    if [[ "${TRAVIS_BRANCH:-}" == features/* ]]; then
        NPM_TAG=$(echo "${TRAVIS_BRANCH}" | sed -e 's|^features/|feature-|g')
    fi

    # If the package doesn't have an alpha tag, use the tag of latest instead of
    # dev. NPM uses this tag as the default version to add, so we want it to mean
    # the newest released version.
    if [[ $(jq -r .version < "${ROOT}/sdk/nodejs/bin/package.json") != *-alpha* ]]; then
        NPM_TAG="latest"
    fi

    pushd "${ROOT}/sdk/nodejs/bin" && \
        npm publish --tag "${NPM_TAG}" && \
        npm info 2>/dev/null || true && \
        popd

    echo "Publishing Pip package to pulumi.com:"
    twine upload \
        -u pulumi -p "${PYPI_PASSWORD}" \
        "${ROOT}/sdk/python/env/src/dist"/*.whl

    "${ROOT}/scripts/build-and-publish-docker" "${NPM_VERSION}"

    "${ROOT}/scripts/build-docs.sh"
fi

exit 0
