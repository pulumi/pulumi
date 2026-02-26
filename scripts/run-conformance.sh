#!/usr/bin/env bash

set -euox pipefail

ROOT=$(git rev-parse --show-toplevel)

# When developing conformance tests it's quite common to need to run the same test over and over for all the languages.
# This script is a helper to do that. It takes a single argument which is the name of the test to run and goes and runs
# that test for each language.

# We nearly always want the full output when developing tests.
export PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT=true

# Run *all* language tests
if [ "$1" = "" ]; then
    cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage"

    cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguageTSC"
    cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguageTSNode"

    cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageDefault"
    cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageTOML"
    cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageClasses"

    exit 0
fi

cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/.*/$1"

cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguageTSC/.*/$1"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguageTSNode/local=false/$1"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguageTSNode/.*/$1"

cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageDefault/.*/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageTOML/.*/$1"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageClasses/.*/$1"
