PROJECT_NAME     := Pulumi Go SDK
PROVIDER_PKG     := github.com/pulumi/pulumi/providers/pulumi-resource-inline
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh go))

ifeq ($(DEBUG),"true")
$(info    VERSION         = $(VERSION))
endif

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= install

ensure: .make/ensure/pulumi-resource-inline

# .PHONY: ../../bin/pulumi-resource-inline
# ../../bin/pulumi-resource-inline:
# 	go build -o ../../bin/pulumi-resource-inline \
# 		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
# 		${PROVIDER_PKG}
.PHONY: ../../bin/pulumi-resource-inline
../../bin/pulumi-resource-inline:
	go build -o ../../bin/pulumi-resource-inline

.PHONY: install_plugin
install_plugin: ../../bin/pulumi-resource-inline
	cp $< $(PULUMI_BIN)/pulumi-resource-inline

install:: install_plugin

test_all:: test_fast

test_fast:: $(TEST_ALL_DEPS)
	@$(GO_TEST_FAST) $(TEST_FAST_PKGS)

	$(GO_TEST_FAST) $(shell go list ./... | grep -v /vendor/ | grep -v templates)

dist:: ../../bin/pulumi-resource-inline
ifneq (${GOBIN},)
	@# We need to do this dance instead of just calling `cp` because `make build`
	@# (from root) sets ${GOBIN} such that the copy is a no op.
	@bash -c ' \
	if ! [[ "$<" -ef ${GOBIN}/pulumi-resource-inline ]]; then \
		echo cp -f $< ${GOBIN}/pulumi-resource-inline; \
		cp $< ${GOBIN}/pulumi-resource-inline; \
	fi'
else
	cp -f $< $(shell go env GOPATH)/bin/pulumi-resource-inline
endif

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
# brew::
# 	go install -C pulumi-resource-inline \
# 		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" \
# 		${PROVIDER_PKG}
brew::
	go install ${PROVIDER_PKG}

lint:: .make/ensure/golangci-lint
	golangci-lint run -c ../../.golangci.yml --timeout 5m --path-prefix pulumi-resource-inline
