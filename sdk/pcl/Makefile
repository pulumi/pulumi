PROJECT_NAME     := Pulumi pcl SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/pcl/v3/cmd/pulumi-language-pcl
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh pcl))

include ../../build/common.mk

ensure: .make/ensure/go

build_plugin: ../../bin/pulumi-language-pcl

.PHONY: ../../bin/pulumi-language-pcl
../../bin/pulumi-language-pcl:
	go build -C cmd/pulumi-language-pcl \
	-o ../../$@ \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
		${LANGHOST_PKG}

build:: build_plugin

install_plugin:: build
	GOBIN=$(PULUMI_BIN) go install -C cmd/pulumi-language-pcl \
	   -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

dist:: GOBIN=$(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
dist::
	go install -C cmd/pulumi-language-pcl \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -C cmd/pulumi-language-pcl \
	   -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

lint:: .make/ensure/golangci-lint
	cd .. && golangci-lint run -c ../.golangci.yml --timeout 5m