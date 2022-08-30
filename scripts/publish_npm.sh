#!/bin/bash
# publish_npm.sh uploads our packages to npm
set -o nounset
set -o errexit
set -o pipefail
readonly ROOT=$(dirname "${0}")/..

echo "Publishing NPM package to NPMjs.com:"
NPM_TAG="latest"

PKG_NAME="@transcend-io/pulumi"
PKG_VERSION="1.0.23"

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
npm publish -tag "${NPM_TAG}" --access=public
npm info 2>/dev/null
popd
