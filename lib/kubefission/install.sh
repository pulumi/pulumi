#!/bin/bash
# A simple installation script for the Coconut Fission library.

set -e                    # bail on errors

echo Compiling:
pushd pack/ > /dev/null &&    # compile the package
    cocojs &&
    coco pack verify &&       # ensure the package verifies
    yarn link &&              # let NPM references resolve easily.
    cp -R ./.coconut/bin ../bin &&
    popd > /dev/null
pushd provider/ >/dev/null && # compile the resource provider
    go build -o ../bin/coco-resource-kubefission &&
    popd > /dev/null

COCOLIB=/usr/local/coconut/lib
THISLIB=$COCOLIB/kubefission/
echo Installing Coconut Fission library to $THISLIB:
mkdir -p $COCOLIB             # ensure the target library directory exists
rm -rf $THISLIB               # clean the target
cp -R ./bin/ $THISLIB         # copy to the standard library location

echo Done.

