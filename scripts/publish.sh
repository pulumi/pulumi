#!/bin/bash
# publish.sh builds and publishes a release.
set -e

PUBLISH=$GOPATH/src/github.com/pulumi/home/scripts/publish.sh
if [ ! -f $PUBLISH ]; then
    >&2 echo "error: Missing publish script at $PUBLISH"
    exit 1
fi

RELEASE_INFO=($($(dirname $0)/make_release.sh))
${PUBLISH} ${RELEASE_INFO[0]} pulumi ${RELEASE_INFO[@]:1}

