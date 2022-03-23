PROJECT_NAME     := Pulumi Go SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v3/go/pulumi-language-go
VERSION          := $(shell cd ../../ && pulumictl get version)
TEST_FAST_PKGS   := $(shell go list ./pulumi/... ./pulumi-language-go/... ./common/... | grep -v /vendor/ | grep -v templates)
TEST_AUTO_PKGS   := $(shell go list ./auto/... | grep -v /vendor/ | grep -v templates)
TESTPARALLELISM  ?= 10
PROJECT_ROOT     := $(realpath ../..)

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= install

gen::
	go generate ./pulumi/...

build:: gen
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

test_all:: test_fast test_auto

test_fast:: $(TEST_ALL_DEPS)
	@$(GO_TEST_FAST) $(TEST_FAST_PKGS)

test_auto:: $(TEST_ALL_DEPS)
	@$(GO_TEST) $(TEST_AUTO_PKGS)

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}

lint::
	cd .. && golangci-lint run -c ../.golangci.yml --timeout 5m
