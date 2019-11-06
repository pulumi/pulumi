PROJECT_NAME         := Pulumi .NET Core SDK
LANGHOST_PKG         := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet

PROJECT_PKGS         := $(shell go list ./cmd...)

# VERSION will be something like: v1.5.0-alpha.1573070062+g76d79be4
VERSION              := $(shell ../../scripts/get-version HEAD --preview)

# strip v from the beginning to get: 1.5.0-alpha.1573070062+g76d79be4
VERSION_DOTNET       := ${VERSION:v%=%}

# Grab: 1.5.0
VERSION_FIRST_WORD   := $(strip $(word 1,$(subst -, ,${VERSION_DOTNET})))

# Grab: -alpha.1573070062+g76d79be4
VERSION_SECOND_WORD  := $(strip $(word 2,$(subst -, ,${VERSION_DOTNET})))

VERSION_PREFIX := ${VERSION_FIRST_WORD}

# .NET support is only in Preview currently.  So unilaterally add `-timestamp+ghash` to the version
# suffix. From the nuget docs:
#
# Pre-release versions are then denoted by appending a hyphen and a string after the patch number.
# Technically speaking, you can use any string after the hyphen and NuGet will treat the package as
# pre-release. NuGet then displays the full version number in the applicable UI, leaving consumers
# to interpret the meaning for themselves.
VERSION_SUFFIX := -${VERSION_SECOND_WORD}

TESTPARALLELISM := 10

include ../../build/common.mk

build::
	dotnet build dotnet.sln /p:VersionPrefix=${VERSION_PREFIX} /p:VersionSuffix=${VERSION_SUFFIX}
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
	# work around a bug where running `dotnet test` disables console stdinput display
	stty echo

test_fast:: dotnet_test
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: dotnet_test
	$(GO_TEST) ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
