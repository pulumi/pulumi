#!/usr/bin/env bash

set -eo pipefail
set -x

PROJECT="$1"
BREW_VERSION=$(./scripts/get-version HEAD)

# Rebuild and install pulumi CLI binaries into $GOPATH/bin
(cd pkg && go install \
              -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${BREW_VERSION}" \
              ${PROJECT})

# Fetch extra language binaries like YAML and Java from GitHub releases.
./scripts/get-language-providers.sh

# Install these extra binaries into $GOPATH/bin
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
GOPATH=$(go env GOPATH)
# goreleaser in pulumi/pulumi renames amd64 to x64
RENAMED_ARCH="${GOARCH/amd64/x64}"
mkdir -p "$GOPATH/bin"
cp goreleaser-lang/*/${GOOS}-${RENAMED_ARCH}/pulumi-language-* "$GOPATH/bin/"
