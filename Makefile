PROJECT_NAME := Pulumi Fabric
SUB_PROJECTS := sdk/nodejs
include build/common.mk

PROJECT         := github.com/pulumi/pulumi
PROJECT_PKGS    := $(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
EXTRA_TEST_PKGS := $(shell go list ./examples/ ./tests/... | grep -v /vendor/)
VERSION         := $(shell git describe --tags --dirty 2>/dev/null)

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=Gometalinter.json

TESTPARALLELISM := 10

# Our travis workers are a little show and sometime the fast tests take a little longer
ifeq ($(TRAVIS),true)
TEST_FAST_TIMEOUT := 10m
else
TEST_FAST_TIMEOUT := 2m
endif

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

LINT_SUPPRESS="or be unexported"
lint::
	$(GOMETALINTER) main.go | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./pkg/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./cmd/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))

test_fast::
	go test -timeout $(TEST_FAST_TIMEOUT) -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

test_all::
	PATH=$(PULUMI_ROOT)/bin:$(PATH) go test -cover -parallel ${TESTPARALLELISM} ${EXTRA_TEST_PKGS}

.PHONY: publish
publish:
	$(call STEP_MESSAGE)
	./scripts/publish.sh

.PHONY: coverage
coverage:
	$(call STEP_MESSAGE)
	./scripts/gocover.sh

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all coverage
travis_push: only_build publish only_test
travis_pull_request: all
travis_api: all
