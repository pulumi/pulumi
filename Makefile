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

	@echo "cd sdk && go mod download"; cd sdk && go mod download
	@echo "cd pkg && go mod download"; cd pkg && go mod download
	@echo "cd tests && go mod download"; cd tests && go mod download

build-proto::
	cd sdk/proto && ./generate.sh

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "This command does not do anything anymore. It will be removed in a future version."

build::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

build_debug::
	cd pkg && go install -gcflags="all=-N -l" -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

developer_docs::
	cd developer-docs && make html

install::
	cd pkg && GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

install_all:: install

dist:: build
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

# NOTE: the brew target intentionally avoids the dependency on `build`, as it does not require the language SDKs.
brew:: BREW_VERSION := $(shell scripts/get-version HEAD)
brew::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${BREW_VERSION}" ${PROJECT}

lint::
	@[ "$(shell cd pkg && golangci-lint run -c ../.golangci.yml --timeout 5m 1>&2; echo $$?)" -eq 0 ] && \
	[ "$(shell cd sdk && golangci-lint run -c ../.golangci.yml --timeout 5m 1>&2; echo $$?)" -eq 0 ] && \
	[ "$(shell cd tests && golangci-lint run -c ../.golangci.yml --timeout 5m 1>&2; echo $$?)" -eq 0 ]

test_fast:: build
	cd pkg && $(GO_TEST_FAST) ${PROJECT_PKGS}

test_build:: $(TEST_ALL_DEPS)
	cd tests/testprovider && go build -o pulumi-resource-testprovider
	cd tests/integration/construct_component/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_slow/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_plain/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_plain/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_unknown/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_unknown/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/component_provider_schema/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/component_provider_schema/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_error_apply/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_methods/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_methods/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_provider/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_provider/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_methods_unknown/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_methods_unknown/testcomponent-go && go build -o pulumi-resource-testcomponent
	cd tests/integration/construct_component_methods_resources/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc
	cd tests/integration/construct_component_methods_resources/testcomponent-go && go build -o pulumi-resource-testcomponent

test_all:: test_build
	cd pkg && $(GO_TEST) ${PROJECT_PKGS}
	cd tests && $(GO_TEST) -p=1 ${TESTS_PKGS}
