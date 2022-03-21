#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

# TODO the ignored test seems to fail in pytest but not unittest. Need
# to trackdown why.

coverage run -m pytest lib/test \
             --ignore lib/test/automation \
             --ignore lib/test/langhost/resource_thens/test_resource_thens.py

coverage run -m unittest \
             lib/test/langhost/resource_thens/test_resource_thens.py

# Using python -m also adds lib/test_with_mocks to sys.path which
# avoids package resolution issues.

(cd lib/test_with_mocks && coverage run -m pytest)

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-fast.xml
    fi
fi
