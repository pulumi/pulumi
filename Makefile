PROJECT_NAME := Pulumi SDK
PROJECT_ROOT := $(realpath .)
SUB_PROJECTS := sdk/dotnet sdk/nodejs sdk/python sdk/go

include build/common.mk

PROJECT         := github.com/pulumi/pulumi/pkg/v3/cmd/pulumi
PROJECT_PKGS    := $(shell cd ./pkg && go list ./... | grep -v /vendor/)
TESTS_PKGS      := $(shell cd ./tests && go list -tags all ./... | grep -v tests/templates | grep -v /vendor/)
VERSION         := $(shell pulumictl get version)

TESTPARALLELISM := 10

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS = build $(SUB_PROJECTS:%=%_install)

ensure::
	$(call STEP_MESSAGE)
	@echo "Check for pulumictl"; [ -e "$(shell which pulumictl)" ]

	cd sdk && go mod download
	cd pkg && go mod download
	cd tests && go mod download

build-proto::
	cd sdk/proto && ./generate.sh

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "This command does not do anything anymore. It will be removed in a future version."

ifeq ($(PULUMI_TEST_COVERAGE_PATH),)
build::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

install::
	cd pkg && GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}
else
build:: build_cover ensure_cover

ensure_cover::
	mkdir -p $(PULUMI_TEST_COVERAGE_PATH)

install:: install_cover
endif

build_debug::
	cd pkg && go install -gcflags="all=-N -l" -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

build_cover::
	cd pkg && go test -coverpkg github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/... -cover -c -o $(shell go env GOPATH)/bin/pulumi -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

install_cover:: build_cover
	cp $(shell go env GOPATH)/bin/pulumi $(PULUMI_BIN)

developer_docs::
	cd developer-docs && make html

install_all:: install

dist:: build
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

# NOTE: the brew target intentionally avoids the dependency on `build`, as it does not require the language SDKs.
brew:: BREW_VERSION := $(shell pulumictl get version)
brew::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${BREW_VERSION}" ${PROJECT}

.PHONY: lint_pkg lint_sdk lint_tests
lint:: lint_pkg lint_sdk lint_tests
lint_pkg:
	cd pkg && golangci-lint run -c ../.golangci.yml --timeout 5m
lint_sdk:
	cd sdk && golangci-lint run -c ../.golangci.yml --timeout 5m
lint_tests:
	cd tests && golangci-lint run -c ../.golangci.yml --timeout 5m

test_fast:: build
	cd pkg && $(GO_TEST_FAST) ${PROJECT_PKGS}

test_build:: $(TEST_ALL_DEPS)
	cd tests/testprovider && go build -o pulumi-resource-testprovider$(shell go env GOEXE)
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_output_values
	cd tests/integration/construct_component_slow/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_plain
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_unknown
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh component_provider_schema
	cd tests/integration/construct_component_error_apply/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_methods
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_provider
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_methods_unknown
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_methods_resources
	PYTHON=$(PYTHON) ./scripts/prepare-test.sh construct_component_methods_errors

test_all:: test_build test_pkg test_integration

test_pkg::
	cd pkg && $(GO_TEST) ${PROJECT_PKGS}

test_integration::
	cd tests && $(GO_TEST) -p=1 ${TESTS_PKGS}

tidy::
	./scripts/tidy.sh

validate_codecov_yaml::
	curl --data-binary @codecov.yml https://codecov.io/validate
