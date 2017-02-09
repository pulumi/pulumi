#!/bin/bash
# A simple installation script for the Mu standard library.

set -e                   # bail on errors

echo Compiling:
mujs                     # compile the package

echo Sharing NPM links:
yarn link                # let NPM references resolve easily.

MULIB=/usr/local/mu/lib
echo Installing Mu standard library to $MULIB:
mkdir -p $MULIB          # ensure the target library directory exists
cp -Rv ./bin/ $MULIB/mu/ # copy to the standard library location

echo Done.

