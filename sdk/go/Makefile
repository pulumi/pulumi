PROJECT_NAME     := Pulumi Go SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v3/go/pulumi-language-go
VERSION          := $(shell cd ../../ && pulumictl get version)
PROJECT_PKGS     := $(shell go list ./pulumi/... ./pulumi-language-go/... ./common/... ./auto/...| grep -v /vendor/ | grep -v templates)

TESTPARALLELISM := 10

include ../../build/common.mk

gen::
	go generate ./pulumi/...

build:: gen
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

test_all:: test_fast

test_fast:: install
	go test -count=1 -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}
