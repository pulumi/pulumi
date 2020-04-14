PROJECT_NAME     := Pulumi Go SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v2/go/pulumi-language-go
VERSION          := $(shell ../../scripts/get-version HEAD)
PROJECT_PKGS     := $(shell go list ./pulumi/... ./pulumi-language-go/... ./common/...| grep -v /vendor/ | grep -v templates)

TESTPARALLELISM := 10

include ../../build/common.mk

gen::
	go generate ./pulumi/...

build:: gen
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

test_all:: test_fast

test_fast::
	go test -count=1 -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
