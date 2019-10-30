PROJECT_NAME         := Pulumi .NET Core SDK
LANGHOST_PKG         := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet

PROJECT_PKGS         := $(shell go list ./cmd...)

VERSION              := $(shell ../../scripts/get-version HEAD --embed-feature-branch)
VERSION_DOTNET       := ${VERSION:v%=%}                                   # strip v from the beginning
VERSION_FIRST_WORD   := $(strip $(word 1,$(subst -, ,${VERSION_DOTNET}))) # e.g. 1.5.0
VERSION_SECOND_WORD  := $(strip $(word 2,$(subst -, ,${VERSION_DOTNET}))) # e.g. alpha or alpha.1
VERSION_THIRD_WORD   := $(strip $(word 3,$(subst -, ,${VERSION_DOTNET}))) # e.g. featbranch or featbranch.1

VERSION_PREFIX       := ${VERSION_FIRST_WORD}

ifeq ($(VERSION_THIRD_WORD),)
	VERSION_SUFFIX   := ${VERSION_SECOND_WORD}
else
	# have to use addsuffix or make adds a whitespace between the two values.
	VERSION_SUFFIX   := $(addsuffix -$(VERSION_THIRD_WORD),${VERSION_SECOND_WORD})
endif

TESTPARALLELISM := 10

include ../../build/common.mk

build::
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
	dotnet build dotnet.sln /p:VersionPrefix=${VERSION_PREFIX} /p:VersionSuffix=preview-${VERSION_SUFFIX}
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_plugin
	echo "Copying NuGet packages to ${PULUMI_NUGET}"
	[ ! -e "$(PULUMI_NUGET)" ] || rm -rf "$(PULUMI_NUGET)/*"
	find . -name '*.nupkg' -exec cp -p {} ${PULUMI_NUGET} \;

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
