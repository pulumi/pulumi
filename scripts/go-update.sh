#!/usr/bin/env bash

set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
echo "$ROOT"
# See: https://www.shellcheck.net/wiki/SC2044
find "$ROOT" -mindepth 3 -name 'go.mod' -print0 | while IFS= read -r -d '' f
do
    (
        cd "$(dirname $f)"

        go get -u github.com/pulumi/pulumi/sdk/v3
        go get -u github.com/pulumi/pulumi/pkg/v3
        go mod tidy -go=1.18 -compat=1.18
    )
done
