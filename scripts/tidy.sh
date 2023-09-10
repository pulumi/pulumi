#!/usr/bin/env bash

set -euo pipefail

for f in $(git ls-files | grep "go\.mod")
do
    (cd "$(dirname "${f}")" && go mod tidy -compat=1.20)
done
