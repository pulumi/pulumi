PROJECT_NAME    := Pulumi .NET Core SDK
LANGHOST_PKG    := github.com/pulumi/pulumi/sdk/v3/dotnet/cmd/pulumi-language-dotnet

PROJECT_PKGS    := $(shell go list ./cmd...)

DOTNET_VERSION  := $(shell cd ../../ && pulumictl get version --language dotnet)

### TO-DO: Automation Version - this will be deleted at 3.0
VERSION_FIRST_WORD   := $(word 1,$(subst -, ,${DOTNET_VERSION})) # e.g. 1.5.0
VERSION_SECOND_WORD  := $(word 2,$(subst -, ,${DOTNET_VERSION})) # e.g. alpha or alpha.1
VERSION_THIRD_WORD   := $(word 3,$(subst -, ,${DOTNET_VERSION})) # e.g. featbranch or featbranch.1

PREVIEW_VERSION_PREFIX       := $(strip ${VERSION_FIRST_WORD})

ifeq ($(strip ${VERSION_SECOND_WORD}),)
	VERSION_SUFFIX   := ""
	PREVIEW_VERSION_SUFFIX := preview
else ifeq ($(strip ${VERSION_THIRD_WORD}),)
	VERSION_SUFFIX   := $(strip ${VERSION_SECOND_WORD})
	PREVIEW_VERSION_SUFFIX := ${VERSION_SUFFIX}
else
	VERSION_SUFFIX   := $(strip ${VERSION_THIRD_WORD})-$(strip ${VERSION_SECOND_WORD})
	PREVIEW_VERSION_SUFFIX := ${VERSION_SUFFIX}
endif

TESTPARALLELISM := 10

include ../../build/common.mk

build::
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
	#dotnet clean
	dotnet build dotnet.sln /p:Version=${DOTNET_VERSION}
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${DOTNET_VERSION}" ${LANGHOST_PKG}
	# Automation is pre-release, so build it separately with a different version to produce a preview NuGet package
	dotnet pack Pulumi.Automation/Pulumi.Automation.csproj /p:VersionPrefix=${PREVIEW_VERSION_PREFIX} /p:VersionSuffix=${PREVIEW_VERSION_SUFFIX}
ifneq (${PREVIEW_VERSION_SUFFIX},${VERSION_SUFFIX})
	rm -f ./Pulumi/bin/*/Pulumi.${PREVIEW_VERSION_PREFIX}-${PREVIEW_VERSION_SUFFIX}.nupkg
endif

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: build install_plugin
	echo "Copying NuGet packages to ${PULUMI_NUGET}"
	[ ! -e "$(PULUMI_NUGET)" ] || rm -rf "$(PULUMI_NUGET)/*"
	rm -f $(PULUMI_NUGET)/*.nupkg
	find . -name '*${VERSION_PREFIX}*.nupkg' -exec cp -p {} ${PULUMI_NUGET} \;

dotnet_test:: install
	# include the version prefix/suffix to avoid generating a separate nupkg file
	dotnet test /p:Version=${DOTNET_VERSION}

test_fast:: dotnet_test
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: dotnet_test
	$(GO_TEST) ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${DOTNET_VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}

publish:: build install
	echo "Publishing .nupkgs to nuget.org:"
	find /opt/pulumi/nuget -name 'Pulumi*.nupkg' \
		-exec dotnet nuget push -k ${NUGET_PUBLISH_KEY} -s https://api.nuget.org/v3/index.json {} ';'

