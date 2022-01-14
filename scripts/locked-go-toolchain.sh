#!/bin/bash

set -euo pipefail

: "${RUNNER_TEMP?Not finding RUNNER_TEMP expected in the GitHub Actions environment}"

mkdir -p "$RUNNER_TEMP/go-wrapper"
(cd build/utils/locked-go && go build -o "$RUNNER_TEMP/go-wrapper/go$(go env GOEXE)")
echo "$(./scripts/normpath "$(command -v go)")" > "$RUNNER_TEMP/go-wrapper/realgo.path"
echo $(./scripts/normpath "$RUNNER_TEMP/go-wrapper")
