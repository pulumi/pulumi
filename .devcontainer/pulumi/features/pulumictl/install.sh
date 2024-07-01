#!/usr/bin/env bash

set -e

version="0.0.42"
arch="amd64"
filename="pulumictl-v$version-linux-$arch.tar.gz"

curl -fsSLO "https://github.com/pulumi/pulumictl/releases/download/v$version/$filename"

tar -xzf "$filename" --directory /usr/local/bin --no-same-owner pulumictl
rm "$filename"

pulumictl version
