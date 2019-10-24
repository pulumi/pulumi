PROJECT_NAME     := Pulumi .NET Core SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet
VERSION          := $(shell ../../scripts/get-version)
PROJECT_PKGS    := $(shell go list ./cmd...)
PULUMI_LOCAL_NUGET ?= ~/.nuget/local

TESTPARALLELISM := 10

include ../../build/common.mk

build::
	dotnet build dotnet.sln
	echo "Copying NuGet packages to ${PULUMI_LOCAL_NUGET}"
	mkdir -p ${PULUMI_LOCAL_NUGET}
	find . -name '*.nupkg' -exec cp -p {} ${PULUMI_LOCAL_NUGET} \;
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin

lint::
	golangci-lint run

dotnet_test::
	dotnet test

test_fast:: dotnet_test
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: dotnet_test
	$(GO_TEST) ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
