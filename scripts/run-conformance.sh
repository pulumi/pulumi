#!/usr/bin/env bash

set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)

# When developing conformance tests it's quite common to need to run the same test over and over for all the languages.
# This script is a helper to do that. It takes a single argument which is the name of the test to run and goes and runs
# that test for each language.

cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run TestLanguage/$1

cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run TestLanguage/forceTsc=false/$1
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run TestLanguage/forceTsc=true/$1

cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run TestLanguage/default/$1
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run TestLanguage/toml/$1
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run TestLanguage/classes/$1