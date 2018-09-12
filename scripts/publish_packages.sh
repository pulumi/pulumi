#!/bin/bash
# publish_packages.sh uploads our packages to package repositories like npm
set -o nounset
set -o errexit
set -o pipefail
readonly ROOT=$(dirname "${0}")/..

if [[ "${TRAVIS_PUBLISH_PACKAGES:-}" == "true" ]]; then
    echo "Publishing NPM package to NPMjs.com:"
    NPM_TAG="dev"

    # If the package doesn't have a pre-release tag, use the tag of latest instead of
    # dev. NPM uses this tag as the default version to add, so we want it to mean
    # the newest released version.
    if [[ $(jq -r .version < "${ROOT}/sdk/nodejs/bin/package.json") != *-* ]]; then
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
fi

"${ROOT}/scripts/build-sdk.sh" $("${ROOT}/scripts/get-version") $(git rev-parse HEAD)

exit 0