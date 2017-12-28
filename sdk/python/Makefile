PROJECT_NAME    := Pulumi Python SDK
PROJECT         := github.com/pulumi/pulumi/sdk/python
LANGHOST_PKG    := ${PROJECT}/cmd/pulumi-langhost-python
VERSION         := $(shell git describe --tags --dirty 2>/dev/null)

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=Gometalinter.json

TESTPARALLELISM   := 10
TEST_FAST_TIMEOUT := 2m

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

lint::
	$(GOMETALINTER) ${LANGHOST_PKG} | sort ; exit $$(($${PIPESTATUS[1]}-1))
