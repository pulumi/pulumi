#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=${PULUMI_TEST_COVERAGE_PATH:-}

set -euo pipefail

mkdir -p ../../junit
JUNIT_DIR=$(realpath ../../junit)

coverage_args=()
ignore_args=(--ignore lib/test/automation)
for arg in "$@"; do
    case "$arg" in
        --coverage)
            coverage_args=(--cov pulumi)
            ;;
        --fast)
            ignore_args+=(--ignore lib/test/langhost)
            ;;
    esac
done
if [[ -n "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    coverage_args=(--cov pulumi)
fi

uv run -m pytest "${coverage_args[@]}" -n auto --junitxml "$JUNIT_DIR/python-test-unit.xml" lib/test \
    "${ignore_args[@]}"

# Start with fresh coverage data above, then append coverage from subsequent test suites.
if [[ ${#coverage_args[@]} -ne 0 ]]; then
    coverage_args+=(--cov-append)
fi

uv run -m pytest "${coverage_args[@]}" --junitxml "$JUNIT_DIR/python-test-unit-with-mocks.xml" lib/test_with_mocks

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        UUID=$(python -c "import uuid; print(str(uuid.uuid4()).replace('-', '').lower())")
        uv run -m coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-fast-$UUID.xml
    fi
fi
