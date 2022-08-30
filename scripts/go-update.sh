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
        go mod tidy -compat=1.17
    )
done

# while IFS= read -r -d ''
# for f in $(find "$ROOT" -mindepth 3 -name 'go.mod')
# do
#     set -x
#     (
#         cd $(dirname $f)
#         go mod tidy -compat=1.17
#     )
#     set +x
# done
