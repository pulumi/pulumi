PROJECT_NAME         := Pulumi .NET Core SDK
LANGHOST_PKG         := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet
VERSION              := $(shell ../../scripts/get-version)
VERSION_DOTNET       := ${VERSION:v%=%}                             # strip v from the beginning
VERSION_PREFIX       := $(firstword $(subst -, ,${VERSION_DOTNET})) # e.g. 1.5.0
VERSION_SUFFIX       := $(word 2,$(subst -, ,${VERSION_DOTNET}))    # e.g. alpha.1
VERSION_SUFFIX_START := $(strip $(firstword $(subst ., ,${VERSION_SUFFIX})))  # e.g. alpha
VERSION_SUFFIX_END   := $(word 2,$(subst ., ,${VERSION_SUFFIX}))     # e.g. 1
PROJECT_PKGS         := $(shell go list ./cmd...)
PULUMI_LOCAL_NUGET   ?= ~/.nuget/local

# .NET support is only in Preview currently.  So unilaterally add `-preview` to the version suffix.
# From the nuget docs:
#
# Pre-release versions are then denoted by appending a hyphen and a string after the patch number.
# Technically speaking, you can use any string after the hyphen and NuGet will treat the package as
# pre-release. NuGet then displays the full version number in the applicable UI, leaving consumers
# to interpret the meaning for themselves.
#
# With this in mind, it's generally good to follow recognized naming conventions such as the
# following:
#
#     -alpha: Alpha release, typically used for work-in-progress and experimentation
FINAL_VERSION_SUFFIX := $(addsuffix -preview.${VERSION_SUFFIX_END},${VERSION_SUFFIX_START})

TESTPARALLELISM := 10

include ../../build/common.mk

build::
	dotnet build dotnet.sln /p:VersionPrefix=${VERSION_PREFIX} /p:VersionSuffix=${FINAL_VERSION_SUFFIX}
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
