#!/usr/bin/env bash

set -euo pipefail

bench() {
    hyperfine --prepare "../script/setup-benchmark.sh" \
              --cleanup "pulumi destroy -y" \
              "pulumi up -y"
}

pushd "$1"
pulumi stack rm -y || echo "preparing.."

pulumi stack init benchmark

echo "Control: $(pulumi version)"
bench

export PATH=~/.pulumi-dev/bin:$PATH
yarn link @pulumi/pulumi
yarn install
echo "Test: $(pulumi version)"
bench
yarn unlink @pulumi/pulumi
yarn install --force

pulumi stack rm -y

popd
