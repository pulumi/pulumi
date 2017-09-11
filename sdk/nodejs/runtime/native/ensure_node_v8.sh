#!/bin/sh
# This script ensures that the full distribution of Node.js is available.
set -e

# We need for the internal V8 headers; download the source tarball and unpack it.
NODE_BASE=third_party/node
NODE_TARGET=$NODE_BASE/node-$(node -p "process.version")
if [ -d $NODE_TARGET ]; then
    echo Skipping Node.js/V8 internal sources and headers download, as they already exist
else
    NODE_DISTRO=$(node -p "process.release.sourceUrl")
    echo Downloading Node.js/V8 internal sources and headers from $NODE_DISTRO...
    NODE_TARBALL=$(mktemp)
    curl -s $NODE_DISTRO -o $NODE_TARBALL
    mkdir -p $NODE_BASE
    tar xzf $NODE_TARBALL -C $NODE_BASE
fi

echo "Done; $NODE_TARGET is fully populated."

