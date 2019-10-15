PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/nodejs sdk/python sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi
FAST_TEST_PKGS  := $(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
SLOW_TEST_PKGS  := $(shell go list ./examples/ ./tests/... | grep -v tests/templates | grep -v /vendor/)
TEMPLATES_PKGS  := $(shell go list ./tests/templates)
VERSION         := $(shell scripts/get-version)

TESTPARALLELISM := 20

build-proto::
	time cd sdk/proto && ./generate.sh

build::
	time go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

install::
	time GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

dist::
	time go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

lint::
	time golangci-lint run

test_fast::
	$(GO_TEST_FAST) ${FAST_TEST_PKGS}

test_all::
	$(GO_TEST) ${FAST_TEST_PKGS}
	$(GO_TEST) ${SLOW_TEST_PKGS}

test_templates::
	$(GO_TEST) -v ${TEMPLATES_PKGS}

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
travis_cron: all coverage test_templates
travis_push: only_build publish_tgz only_test publish_packages
travis_pull_request: all
travis_api: all
