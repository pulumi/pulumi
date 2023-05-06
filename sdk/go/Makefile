PROJECT_NAME     := Pulumi Go SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh go))
TEST_FAST_PKGS   := $(shell go list ./pulumi/... ./common/... | grep -v /vendor/ | grep -v templates)
TEST_AUTO_PKGS   := $(shell go list ./auto/... | grep -v /vendor/ | grep -v templates)

ifeq ($(DEBUG),"true")
$(info    VERSION         = $(VERSION))
endif

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= install

GO_TEST      = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FLAGS)
GO_TEST_FAST = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FAST_FLAGS)

gen::
	go generate ./pulumi/...

ensure: go.ensure

build:: gen
	cd pulumi-language-go && \
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	cd pulumi-language-go && \
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

test_all:: test_fast test_auto

test_fast:: $(TEST_ALL_DEPS)
	@$(GO_TEST_FAST) $(TEST_FAST_PKGS)

	@cd pulumi-language-go && $(GO_TEST_FAST) $(shell go list ./... | grep -v /vendor/ | grep -v templates)

test_auto:: $(TEST_ALL_DEPS)
	@$(GO_TEST) $(TEST_AUTO_PKGS)

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	cd pulumi-language-go && \
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}

lint:: golangci-lint.ensure
	cd .. && golangci-lint run -c ../.golangci.yml --timeout 5m

	cd pulumi-language-go && \
	golangci-lint run -c ../../../.golangci.yml --timeout 5m

publish:
	git tag sdk/v${VERSION}
	git push origin sdk/v${VERSION}
