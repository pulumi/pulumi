#!/bin/bash
set -e
set -o nounset -o errexit -o pipefail

NODE_VERSION=v6.10.2
NODE_BASE=custom_node/node
NODE_EXE=$NODE_BASE/node
if [ -f $NODE_EXE ]; then
    echo "skipping node.js executable download, as it already exists"
else
    echo "node.js binary does not exist, downloading..."
    OS=$(go env GOOS)
    aws s3 cp --only-show-errors s3://eng.pulumi.com/releases/pulumi-node/$OS/$NODE_VERSION.tgz $NODE_BASE/$NODE_VERSION.tgz
    TEMPDIR=$(mktemp -d)
    tar -xvzf $NODE_BASE/$NODE_VERSION.tgz -C $TEMPDIR
    cp $TEMPDIR/out/Release/node $NODE_BASE
    rm -rf $TEMPDIR
    rm -f $NODE_BASE/$NODE_VERSION.tgz
fi

echo "done!"