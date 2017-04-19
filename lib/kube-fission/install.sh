#!/bin/bash
# A simple installation script for the Coconut Fission library.

set -e                    # bail on errors

echo Compiling:
cocojs                    # compile the package
pushd provider/ &&        # compile the resource provider
    go build -o ../.coconut/bin/coco-resource-kube-fission &&
    popd

echo Verifying:
coco pack verify          # ensure the package verifies

echo Sharing NPM links:
yarn link                 # let NPM references resolve easily.

COCOLIB=/usr/local/coconut/lib
THISLIB=$COCOLIB/kube-fission/
echo Installing Coconut Fission library to $THISLIB:
mkdir -p $COCOLIB               # ensure the target library directory exists
rm -rf $THISLIB                 # clean the target
cp -Rv ./.coconut/bin/ $THISLIB # copy to the standard library location

echo Done.

