#!/usr/bin/env bash

set -euo pipefail

# Excluding tests that have their dependencies code-generated but under .gitignore.
EXCLUDE="-e tests/integration/construct_component_configure_provider/go/go.mod -e sdk/go/pulumi-language-go/testdata -e tests/integration/go/parameterized/go.mod"

for f in $(git ls-files '**go.mod' | grep -v $EXCLUDE)
do
    (cd "$(dirname "${f}")" && go mod tidy -compat=1.21)
done
