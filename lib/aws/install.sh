#!/bin/bash
# A simple installation script for the Mu standard library.

set -e                    # bail on errors

echo Compiling:
mujs                      # compile the package

echo Sharing NPM links:
yarn link                 # let NPM references resolve easily.

MULIB=/usr/local/mu/lib
THISLIB=$MULIB/mujs/
echo Installing Mu AWS library to $THISLIB:
mkdir -p $MULIB           # ensure the target library directory exists
rm -rf $THISLIB           # clean the target
cp -Rv ./bin/ $THISLIB    # copy to the standard library location

echo Done.

