#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PULUMI_ROOT=$(dirname $SCRIPT_DIR)

if ! command -v gotestsum &> /dev/null
then
    go test "$@"
else
    mkdir -p $PULUMI_ROOT/test-results/
    TESTRUN=$RANDOM
    gotestsum --jsonfile $PULUMI_ROOT/test-results/$TESTRUN.json --junitfile $PULUMI_ROOT/test-results/$TESTRUN.xml -- "$@"
fi
