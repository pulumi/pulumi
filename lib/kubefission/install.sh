#!/bin/bash
# A simple installation script for the Lumi Fission library.

set -e                    # bail on errors

echo Compiling:
pushd pack/ > /dev/null &&    # compile the package
    lumijs &&
    lumi pack verify &&       # ensure the package verifies
    yarn link &&              # let NPM references resolve easily.
    cp -R ./.lumi/bin ../bin &&
    popd > /dev/null
pushd provider/ >/dev/null && # compile the resource provider
    go build -o ../bin/lumi-resource-kubefission &&
    popd > /dev/null

LUMILIB=/usr/local/lumi/lib
THISLIB=$LUMILIB/kubefission/
echo Installing Lumi Fission library to $THISLIB:
mkdir -p $LUMILIB             # ensure the target library directory exists
rm -rf $THISLIB               # clean the target
cp -R ./bin/ $THISLIB         # copy to the standard library location

echo Done.

