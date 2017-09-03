#!/bin/sh
# This script ensures that the full distribution of Node.js is available.
NODE_VERSION=$(node -p "process.version.substring(1)")
NODE_TP=third_party/node
NODE_TARGET=$NODE_TP/$NODE_VERSION
mkdir -p $NODE_TARGET

# node-gyp will have already downloaded the headers tarball; use it.
if [ -f $NODE_TP/.headers ]; then
    echo Skipping Node.js/V8 header copying, as they already exist
else
    NODE_HEADERS=~/.node-gyp/$NODE_VERSION
    echo Copying Node.js/V8 headers from $NODE_HEADERS...
    [ -d $NODE_HEADERS ] || { >&2 echo "error: Missing Node.js/V8 headers at $NODE_HEADERS"; exit 1; }
    cp -R $NODE_HEADERS/* $NODE_TARGET/
    touch $NODE_TP/.headers
fi

# but it will not have gotten the full sources, which we need for the internal V8 headers; download those.
if [ -f $NODE_TP/.sources ]; then
    echo Skipping Node.js/V8 internal sources and headers download, as they already exist
else
    NODE_DISTRO=$(node -p "process.release.sourceUrl")
    echo Downloading Node.js/V8 internal sources and headers from $NODE_DISTRO...
    NODE_TARBALL=$(mktemp)
    wget -qO $NODE_TARBALL $NODE_DISTRO
    tar xzf $NODE_TARBALL -C $NODE_TARGET
    touch $NODE_TP/.sources
fi

echo "Done; $NODE_TARGET is fully populated."

