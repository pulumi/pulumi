#!/usr/bin/env bash

PULUMI_TEST_COVERAGE_PATH=$PULUMI_TEST_COVERAGE_PATH

set -euo pipefail

coverage run -m pytest lib/test/automation

if [[ "$PULUMI_TEST_COVERAGE_PATH" ]]; then
    if [ -e .coverage ]; then
        UUID=$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-')
        coverage xml -o $PULUMI_TEST_COVERAGE_PATH/python-auto-$UUID.xml
    fi
fi
