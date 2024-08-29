#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

coverage run -m pytest lib/test/automation

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        TS=$(python -c 'import time; print(int(time.time() * 1000))')
        coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-auto-$TS.xml
    fi
fi
