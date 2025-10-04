#!/usr/bin/env bash

set -euox pipefail

ROOT=$(git rev-parse --show-toplevel)

# When developing conformance tests it's quite common to need to run the same test over and over for all the languages.
# This script is a helper to do that. It takes a single argument which is the name of the test to run and goes and runs
# that test for each language.

cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/local=false/$1"
cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/local=true/$1"

cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=false/forceTsc=false/$1"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=false/forceTsc=true/$1"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=true/forceTsc=false/$1"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=true/forceTsc=true/$1"

cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=false/default/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=false/toml/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=false/classes/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=true/default/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=true/toml/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguage/local=true/classes/$1"
