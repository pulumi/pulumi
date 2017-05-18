#!/bin/bash
# A simple installation script for the LumiJS runtime library.

set -e                       # bail on errors

echo Compiling:
lumijs                       # compile the package

echo Verifying:
lumi pack verify             # ensure the package verifies

echo Sharing NPM links:
yarn link                    # let NPM references resolve easily.

LUMILIB=/usr/local/lumi/lib
THISLIB=$LUMILIB/lumijs/
echo Installing LumiJS runtime library to $THISLIB:
mkdir -p $LUMILIB            # ensure the target library directory exists
rm -rf $THISLIB              # clean the target.
cp -Rv ./.lumi/bin/ $THISLIB # copy to the standard library location

echo Done.

