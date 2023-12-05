#!/usr/bin/env bash

set -euo pipefail

# Excluding tests that have their dependencies code-generated but under .gitignore.
EXCLUDE="tests/integration/construct_component_configure_provider/go/go.mod"

for f in $(git ls-files '**go.mod' | grep -v "$EXCLUDE")
do
    (cd "$(dirname "${f}")" && go mod tidy -compat=1.20)
done
