#!/usr/bin/env bash
# publish_npm.sh uploads our packages to npm
set -euo pipefail

readonly ROOT=$(dirname "${0}")/..

echo "Publishing NPM package to NPMjs.com:"
NPM_TAG="dev"

## We need split the GIT_REF into the correct parts
## so that we can test for NPM Tags
IFS='/' read -ra my_array <<< "${GIT_REF:-}"
last_index=$((${#my_array[@]} - 1))
BRANCH_NAME="${my_array[last_index]}"

echo $BRANCH_NAME
if [[ "${BRANCH_NAME}" == features/* ]]; then
    NPM_TAG=$(echo "${BRANCH_NAME}" | sed -e 's|^features/|feature-|g')
fi

if [[ "${BRANCH_NAME}" == feature-* ]]; then
    NPM_TAG="${BRANCH_NAME}"
fi

PKG_NAME=$(jq -r .name < "${ROOT}/sdk/nodejs/package.json")
# shellcheck disable=SC2154 # assigned by release.yml
PKG_VERSION="${PULUMI_VERSION}"

# If the package isn't a dev version (includes the git describe output in its
# version string), use the tag of latest instead of
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
set -x
if [ "$(npm info "${PKG_NAME}@${PKG_VERSION}")" == "" ]; then
    if ! npm publish -tag "${NPM_TAG}" "${ROOT}"/artifacts/sdk-nodejs-*.tgz ; then
    # if we get here, we have a TOCTOU issue, so check again
    # to see if it published. If it didn't bail out.
        if [ "$(npm info "${PKG_NAME}@${PKG_VERSION}")" == "" ]; then
            echo "NPM publishing failed, aborting"
            exit 1
        fi
    fi
fi
