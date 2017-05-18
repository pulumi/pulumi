#!/bin/bash
# A simple installation script for the Lumi AWS library.

set -e                    # bail on errors

echo Compiling:
pushd pack/ > /dev/null &&     # compile the package
    cljs &&
    lumi pack verify &&        # ensure the package verifies.
    yarn link &&               # let NPM references resolve easily.
    cp -R ./.lumi/bin/ ../bin &&
    popd > /dev/null
pushd provider/ > /dev/null && # compile the resource provider
    go build -o ../bin/lumi-resource-aws &&
    popd > /dev/null

LUMILIB=/usr/local/lumi/lib
THISLIB=$LUMILIB/aws/
echo Installing Lumi AWS library to $THISLIB:
mkdir -p $LUMILIB              # ensure the target library directory exists
rm -rf $THISLIB                # clean the target
cp -R ./bin/ $THISLIB          # copy to the standard library location

echo Done.

