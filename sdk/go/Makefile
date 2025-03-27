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

.PHONY: gen
gen:
	go generate ./pulumi/...

ensure: .make/ensure/go

build:: gen ../../bin/pulumi-language-go

.PHONY: ../../bin/pulumi-language-go
../../bin/pulumi-language-go:
	go build -C pulumi-language-go -o ../../../bin/pulumi-language-go \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
		${LANGHOST_PKG}

.PHONY: install_plugin
install_plugin: ../../bin/pulumi-language-go
	cp $< $(PULUMI_BIN)/pulumi-language-go

install:: install_plugin

test_all:: test_fast test_auto

test_fast:: $(TEST_ALL_DEPS)
	@$(GO_TEST_FAST) $(TEST_FAST_PKGS)

	@cd pulumi-language-go && \
		$(GO_TEST_FAST) $(shell cd pulumi-language-go && go list ./... | grep -v /vendor/ | grep -v templates)

test_auto:: $(TEST_ALL_DEPS)
	@$(GO_TEST) $(TEST_AUTO_PKGS)

dist:: ../../bin/pulumi-language-go
ifneq (${GOBIN},)
	@# We need to do this dance instead of just calling `cp` because `make build_local`
	@# (from root) sets ${GOBIN} such that the copy is a no op.
	@if ! [[ "$<" -ef ${GOBIN}/pulumi-language-go ]]; then \
	echo cp -f $< ${GOBIN}/pulumi-language-go; \
	cp $< ${GOBIN}/pulumi-language-go; \
	fi
else
	cp -f $< $(shell go env GOPATH)/bin/pulumi-language-go
endif

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -C pulumi-language-go \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}

lint:: .make/ensure/golangci-lint
	cd .. && golangci-lint run -c ../.golangci.yml --timeout 5m --path-prefix ..

	cd pulumi-language-go && \
		golangci-lint run -c ../../../.golangci.yml --timeout 5m --path-prefix pulumi-language-go

publish:
	git tag sdk/v${VERSION}
	git push origin sdk/v${VERSION}
