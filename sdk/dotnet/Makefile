PROJECT_NAME  := Pulumi dotnet SDK
LANGHOST_PKG  := github.com/pulumi/pulumi/sdk/dotnet/cmd/pulumi-language-dotnet
VERSION       := $(shell ../../scripts/get-version)
VERSIONPREFIX := $(shell echo $(VERSION) | cut -d- -f1 | cut --complement -c1)
VERSIONSUFFIX := $(shell echo $(VERSION) | cut -d- --complement -f1)
PROJECT_PKGS  := $(shell go list ./cmd...)

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

TESTPARALLELISM := 10

include ../../build/common.mk

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
	dotnet build

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
	dotnet pack -c Release /p:VersionPrefix=$(VERSIONPREFIX) /p:VersionSuffix=$(VERSIONSUFFIX)

lint::
	$(GOMETALINTER) ./cmd/pulumi-language-dotnet/... | sort ; exit $${PIPESTATUS[0]}

test_fast::
	go test -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}
