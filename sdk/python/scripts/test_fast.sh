#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

mkdir -p ../../junit
JUNIT_DIR=$(realpath ../../junit)

coverage run --append -m pytest --junitxml "$JUNIT_DIR/python-test-fast.xml" lib/test \
             --ignore lib/test/automation

# Using python -m also adds lib/test_with_mocks to sys.path which
# avoids package resolution issues.

(cd lib/test_with_mocks && coverage run --append -m pytest --junitxml "$JUNIT_DIR/python-test-fast-with-mocks.xml")

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        UUID=$(python -c "import uuid; print(str(uuid.uuid4()).replace('-', '').lower())")
        coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-fast-$UUID.xml
    fi
fi
