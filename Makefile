PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/nodejs sdk/python sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi
PROJECT_PKGS    := $(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
EXTRA_TEST_PKGS := $(shell go list ./examples/ ./tests/... | grep -v /vendor/)
VERSION         := $(shell scripts/get-version)

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=Gometalinter.json

TESTPARALLELISM := 10

# Our travis workers are a little show and sometime the fast tests take a little longer
ifeq ($(TRAVIS),true)
TEST_FAST_TIMEOUT := 10m
else
TEST_FAST_TIMEOUT := 2m
endif

build-proto::
	cd sdk/proto && ./generate.sh

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

LINT_SUPPRESS="or be unexported"
lint::
	$(GOMETALINTER) main.go | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./pkg/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./cmd/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))

test_fast::
	go test -timeout $(TEST_FAST_TIMEOUT) -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

test_all::
	PATH=$(PULUMI_BIN):$(PATH) go test -cover -parallel ${TESTPARALLELISM} ${EXTRA_TEST_PKGS}

.PHONY: publish_tgz
publish_tgz:
	$(call STEP_MESSAGE)
	./scripts/publish_tgz.sh

.PHONY: publish_packages
publish_packages:
	$(call STEP_MESSAGE)
	./scripts/publish_packages.sh

.PHONY: coverage
coverage:
	$(call STEP_MESSAGE)
	./scripts/gocover.sh

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all coverage
travis_push: only_build publish_tgz only_test publish_packages
travis_pull_request: all
travis_api: all
