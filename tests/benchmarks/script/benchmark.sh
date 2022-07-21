#!/usr/bin/env bash

set -euo pipefail

bench() {
    hyperfine -n "$1 pulumi:$(pulumi version) command:'pulumi up -y'" \
              --prepare "../script/setup-benchmark.sh" \
              --cleanup "pulumi destroy -y" \
              "pulumi up -y"
}

pushd "$1"

../script/setup-benchmark.sh
pulumi stack init benchmark

bench control

export PATH=~/.pulumi-dev/bin:$PATH
yarn -s link @pulumi/pulumi
yarn -s install

bench test

yarn -s unlink @pulumi/pulumi
yarn -s install --force
pulumi stack rm -y >/dev/null

popd
