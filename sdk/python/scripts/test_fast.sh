#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

mkdir -p ../../junit
JUNIT_DIR=$(realpath ../../junit)

uv run -m pytest --cov pulumi --cov-append -n auto --junitxml "$JUNIT_DIR/python-test-fast.xml" lib/test \
    --ignore lib/test/automation

uv run -m pytest --cov pulumi --cov-append --junitxml "$JUNIT_DIR/python-test-fast-with-mocks.xml" lib/test_with_mocks

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        UUID=$(python -c "import uuid; print(str(uuid.uuid4()).replace('-', '').lower())")
        uv run -m coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-fast-$UUID.xml
    fi
fi
