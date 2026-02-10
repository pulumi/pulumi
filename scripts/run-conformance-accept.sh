#!/usr/bin/env bash
# Run conformance test with PULUMI_ACCEPT=1 to generate/update snapshot files.
# Usage: ./scripts/run-conformance-accept.sh <test-name>
# Example: ./scripts/run-conformance-accept.sh provider-depends-on-component

set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
export PULUMI_ACCEPT=1
export PULUMI_LANGUAGE_TEST_SHOW_FULL_OUTPUT=true

TEST_NAME="${1:?Usage: $0 <test-name>}"

echo "Running conformance test '$TEST_NAME' with PULUMI_ACCEPT=1..."

# Go skips provider-* tests
cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/published/$TEST_NAME" || true
cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/local/$TEST_NAME" || true
cd "$ROOT/sdk/go/pulumi-language-go" && go test ./... -v -count=1 -run "TestLanguage/extra-types/$TEST_NAME" || true

cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=false/forceTsc=false/$TEST_NAME"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=false/forceTsc=true/$TEST_NAME"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=true/forceTsc=false/$TEST_NAME"
cd "$ROOT/sdk/nodejs/cmd/pulumi-language-nodejs" && go test ./... -v -count=1 -run "TestLanguage/local=true/forceTsc=true/$TEST_NAME"

cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageDefault/local=false/$TEST_NAME"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageTOML/local=false/$TEST_NAME"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageClasses/local=false/$TEST_NAME"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageDefault/local=true/$TEST_NAME"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageTOML/local=true/$TEST_NAME"
cd "$ROOT/sdk/python/cmd/pulumi-language-python" && go test ./... -v -count=1 -run "TestLanguageClasses/local=true/$TEST_NAME"

echo "Done. Check git status for new/modified snapshot files."
