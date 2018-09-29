PROJECT_NAME     := Pulumi Go SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/go/pulumi-language-go
VERSION          := $(shell ../../scripts/get-version)
PROJECT_PKGS     := $(shell go list ./pulumi/... ./pulumi-language-go/... | grep -v /vendor/)

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

TESTPARALLELISM := 10

include ../../build/common.mk

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

lint::
	$(GOMETALINTER) ./pulumi/... | sort
	$(GOMETALINTER) ./pulumi-language-go/... | sort

test_fast::
	go test -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
