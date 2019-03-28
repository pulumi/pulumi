PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/python sdk/nodejs sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi
PROJECT_PKGS    := $(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
EXTRA_TEST_PKGS := $(shell go list ./examples/ ./tests/... | grep -v /vendor/)
VERSION         := $(shell scripts/get-version)

TESTPARALLELISM := 10

build-proto::
	cd sdk/proto && ./generate.sh

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

lint::
	golangci-lint run

test_fast::
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all::
	$(GO_TEST) ${PROJECT_PKGS} ${EXTRA_TEST_PKGS}

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
