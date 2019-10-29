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

    PKG_NAME=$(jq -r .name < "${ROOT}/sdk/nodejs/bin/package.json")
    PKG_VERSION=$(jq -r .version < "${ROOT}/sdk/nodejs/bin/package.json")

    # If the package doesn't have an alpha tag, use the tag of latest instead of
    # dev. NPM uses this tag as the default version to add, so we want it to mean
    # the newest released version.
    if [[ "${PKG_VERSION}" != *-alpha* ]]; then
        NPM_TAG="latest"
    fi

    # Now, perform the publish. The logic here is a little goofy because npm provides
    # no way to say "if the package already exists, don't fail" but we want these
    # semantics (so, for example, we can restart builds which may have failed after
    # publishing, or so two builds can run concurrently, which is the case for when we
    # tag master right after pushing a new commit and the push and tag travis jobs both
    # get the same version.
    #
    # We exploit the fact that `npm info <package-name>@<package-version>` has no output
    # when the package does not exist.
    pushd "${ROOT}/sdk/nodejs/bin"
    if [ "$(npm info ${PKG_NAME}@${PKG_VERSION})" == "" ]; then
        if ! npm publish -tag "${NPM_TAG}"; then
	    # if we get here, we have a TOCTOU issue, so check again
	    # to see if it published. If it didn't bail out.
	    if [ "$(npm info ${PKG_NAME}@${PKG_VERSION})" == "" ]; then
		echo "NPM publishing failed, aborting"
		exit 1
	    fi
	fi
    fi
    npm info 2>/dev/null
    popd

    echo "Publishing Pip package to pulumi.com:"
    twine upload \
        -u pulumi -p "${PYPI_PASSWORD}" \
        "${ROOT}/sdk/python/env/src/dist"/*.whl \
        --skip-existing \
        --verbose

    echo "Publishing .nupkgs to nuget.org:"
    find ~/.nuget/local/ -name '*.nupkg' \
        -exec dotnet nuget push -k ${NUGET_PUBLISH_KEY} -s https://api.nuget.org/v3/index.json {} ';'

    "${ROOT}/scripts/build-and-publish-docker" "${NPM_VERSION}"

    "$(go env GOPATH)/src/github.com/pulumi/scripts/ci/build-package-docs.sh" pulumi
fi

exit 0
