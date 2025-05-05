#!/usr/bin/env bash

# This script manages Go module dependencies across the project using `go mod tidy`.
# It can run in two modes:
# 1. Normal mode: Executes `go mod tidy` on all go.mod files to clean up dependencies
# 2. Check mode (--check flag): Validates go.mod files without modifying them
#    - Checks if any files contain `toolchain` directives
#    - Uses 'go mod tidy -diff' to verify if modules are properly tidied
#    - Fails with non-zero exit code if any issues are found
#
# We don't want to include `toolchain` directives to ensure that CI always run
# with the "current" Go toolchain. Our CI sets up a matrix of versions to use
# and we don't want this to be overriden by the toolchain directive.

set -euo pipefail

# Parse command line arguments
CHECK_MODE=false
for arg in "$@"
do
    case $arg in
        --check)
        CHECK_MODE=true
        shift
        ;;
    esac
done

# Excluding tests that have their dependencies code-generated but under .gitignore.
EXCLUDE="-e tests/integration/construct_component_configure_provider/go/go.mod -e sdk/go/pulumi-language-go/testdata -e tests/testdata -e tests/smoke/testdata"

for f in $(git ls-files '**go.mod' | grep -v $EXCLUDE)
do
    if [ "$CHECK_MODE" = true ]; then
        if grep -q 'toolchain go1.' "$f"; then
            echo "$f contains 'toolchain' directive"
            exit 1
        fi

        if ! (cd "$(dirname "${f}")" && go mod tidy -compat=1.23 -diff); then
            echo "$f is not tidy"
            echo "Please run 'make tidy'"
            exit 1
        fi
    else
        (cd "$(dirname "${f}")" && go mod tidy -compat=1.23)
    fi
done
