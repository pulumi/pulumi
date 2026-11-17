PROJECT_NAME     := Pulumi purejs policy runtime
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/purejs/cmd/pulumi-language-purejs/v3
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh))
LANGHOST_PKGS    := $(shell cd cmd/pulumi-language-purejs && go list ./...)
LDFLAGS          := -X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

include ../../build/common.mk

ensure: .make/ensure/go

build_plugin: ../../bin/pulumi-language-purejs

.PHONY: ../../bin/pulumi-language-purejs
../../bin/pulumi-language-purejs:
	go build -C cmd/pulumi-language-purejs -o ../../$@ -tags grpcnotrace -ldflags "${LDFLAGS}" ${LANGHOST_PKG}

build:: build_plugin

install_plugin:: build
	GOBIN=$(PULUMI_BIN) go install -C cmd/pulumi-language-purejs -tags grpcnotrace -ldflags "${LDFLAGS}" ${LANGHOST_PKG}

install:: install_plugin

test_fast::
	cd cmd/pulumi-language-purejs && $(GO_TEST_FAST) $(LANGHOST_PKGS)

test_all:: $(TEST_ALL_DEPS)
	cd cmd/pulumi-language-purejs && $(GO_TEST) $(LANGHOST_PKGS)

dist:: GOBIN=$(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
dist::
	go install -C cmd/pulumi-language-purejs -tags grpcnotrace -ldflags "${LDFLAGS}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -C cmd/pulumi-language-purejs -tags grpcnotrace -ldflags "${LDFLAGS}" ${LANGHOST_PKG}

