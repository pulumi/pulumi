#!/bin/bash
# publish.sh builds and publishes a release.
set -o nounset -o errexit -o pipefail

PUBLISH=$GOPATH/src/github.com/pulumi/home/scripts/publish.sh
PUBLISH_GOOS=("linux" "darwin")
PUBLISH_GOARCH=("amd64")
PUBLISH_PROJECT="pulumi"

if [ ! -f $PUBLISH ]; then
    >&2 echo "error: Missing publish script at $PUBLISH"
    exit 1
fi

for OS in "${PUBLISH_GOOS[@]}"
do
    for ARCH in "${PUBLISH_GOARCH[@]}"
    do
        export GOOS=${OS}
        export GOARCH=${ARCH}

        RELEASE_INFO=($($(dirname $0)/make_release.sh))
        ${PUBLISH} ${RELEASE_INFO[0]} "${PUBLISH_PROJECT}/${OS}/${ARCH}" ${RELEASE_INFO[@]:1}
    done
done

