#!/bin/bash
# publish.sh builds and publishes the tarballs that our other repositories consume.
set -o nounset
set -o errexit
set -o pipefail

readonly ROOT=$(dirname "${0}")/..
readonly PUBLISH="${GOPATH}/src/github.com/pulumi/scripts/ci/publish.sh"
readonly PUBLISH_GOARCH=("amd64")
readonly PUBLISH_PROJECT="pulumi"

if [[ ! -f "${PUBLISH}" ]]; then
    >&2 echo "error: Missing publish script at $PUBLISH"
    exit 1
fi

readonly OS=$(go env GOOS)

echo "Publishing SDK build to s3://eng.pulumi.com/:"
for ARCH in "${PUBLISH_GOARCH[@]}"; do
    export GOARCH="${ARCH}"
    RELEASE_INFO=($($(dirname "${0}")/make_release.sh))
    "${PUBLISH}" ${RELEASE_INFO[0]} "${PUBLISH_PROJECT}/${OS}/${ARCH}" ${RELEASE_INFO[@]:1}
done

exit 0