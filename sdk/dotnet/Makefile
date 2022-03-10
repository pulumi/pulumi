PROJECT_NAME    := Pulumi .NET Core SDK
LANGHOST_PKG    := github.com/pulumi/pulumi/sdk/v3/dotnet/cmd/pulumi-language-dotnet

PROJECT_PKGS    := $(shell go list ./cmd...)
PROJECT_ROOT    := $(realpath ../..)

DOTNET_VERSION  := $(shell cd ../../ && pulumictl get version --language dotnet)

TESTPARALLELISM ?= 10

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= install

ensure::
	# We want to dotnet restore all projects on startup so that omnisharp doesn't complain about lots of missing types on startup.
	dotnet restore dotnet.sln

ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
TEST_COVERAGE_ARGS := -p:CollectCoverage=true -p:CoverletOutputFormat=cobertura -p:CoverletOutput=$(PULUMI_TEST_COVERAGE_PATH)
else
TEST_COVERAGE_ARGS := -p:CollectCoverage=false -p:CoverletOutput=$(PULUMI_TEST_COVERAGE_PATH)
endif

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
	dotnet clean
	dotnet build dotnet.sln -p:Version=${DOTNET_VERSION}
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${DOTNET_VERSION}" ${LANGHOST_PKG}

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${DOTNET_VERSION}" ${LANGHOST_PKG}

install:: build install_plugin
	echo "Copying NuGet packages to ${PULUMI_NUGET}"
	[ ! -e "$(PULUMI_NUGET)" ] || rm -rf "$(PULUMI_NUGET)/*"
	rm -f $(PULUMI_NUGET)/*.nupkg
	VERSION_PREFIX=${DOTNET_VERSION}; find . -name "*$${VERSION_PREFIX%%+*}*.nupkg" -exec cp -p {} ${PULUMI_NUGET} \;

dotnet_test:: $(TEST_ALL_DEPS)
	$(TESTSUITE_SKIPPED) dotnet-test || \
		dotnet test --filter FullyQualifiedName\!~Pulumi.Automation.Tests -p:Version=${DOTNET_VERSION} ${TEST_COVERAGE_ARGS}/dotnet.xml

test_auto:: $(TEST_ALL_DEPS)
	$(TESTSUITE_SKIPPED) auto-dotnet || \
		dotnet test --filter FullyQualifiedName~Pulumi.Automation.Tests -p:Version=${DOTNET_VERSION} ${TEST_COVERAGE_ARGS}/dotnet-auto.xml

test_fast:: dotnet_test
	@$(GO_TEST_FAST) ${PROJECT_PKGS}

test_go:: $(TEST_ALL_DEPS)
	@$(GO_TEST) ${PROJECT_PKGS}

test_all:: dotnet_test test_auto test_go

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${DOTNET_VERSION}" ${LANGHOST_PKG}

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}

publish:: build install
	echo "Publishing .nupkgs to nuget.org:"
	find $(PULUMI_NUGET) -name 'Pulumi*.nupkg' \
		-exec dotnet nuget push -k ${NUGET_PUBLISH_KEY} -s https://api.nuget.org/v3/index.json {} ';'
