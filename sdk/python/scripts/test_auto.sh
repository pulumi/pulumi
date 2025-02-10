#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

mkdir -p ../../junit
JUNIT_DIR=$(realpath ../../junit)

coverage run --append -m pytest --junitxml "$JUNIT_DIR/python-test-auto.xml" lib/test/automation

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        UUID=$(python -c "import uuid; print(str(uuid.uuid4()).replace('-', '').lower())")
        coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-auto-$UUID.xml
    fi
fi
