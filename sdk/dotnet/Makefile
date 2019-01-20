PROJECT_NAME  := Pulumi dotnet SDK
LANGHOST_PKG  := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet
VERSION       := $(shell ../../scripts/get-version)
VERSIONPREFIX := $(shell echo $(VERSION) | cut -d- -f1 | cut --complement -c1)
VERSIONSUFFIX := $(shell echo $(VERSION) | cut -d- --complement -f1)
PROJECT_PKGS  := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

include ../../build/common.mk

build_package::
	dotnet build

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

install_package::
	dotnet pack -c Release /p:VersionPrefix=$(VERSIONPREFIX) /p:VersionSuffix=$(VERSIONSUFFIX)

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
	
install:: install_package install_plugin

lint::
	golangci-lint run

test_fast::
	go test -count=1 -timeout $(TEST_FAST_TIMEOUT) -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp dist/pulumi-resource-pulumi-dotnet "$$(go env GOPATH)"/bin/