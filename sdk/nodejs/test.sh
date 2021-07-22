#!/usr/bin/env bash

set -euo pipefail

export PATH=$(yarn bin 2>/dev/null):$PATH

./node_modules/.bin/tsc
./node_modules/.bin/istanbul test --print none _mocha -- 'bin/tests_with_mocks/*.spec.js'
