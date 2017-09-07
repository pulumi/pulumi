#!/bin/bash
# install.sh installs an existing release.
set -e

INSTALL=$GOPATH/src/github.com/pulumi/pulumi/scripts/install.sh
if [ ! -f $PUBLISH ]; then
    >&2 echo "error: Missing publish script at $INSTALL"
    exit 1
fi

${INSTALL} pulumi-fabric $1 $2

