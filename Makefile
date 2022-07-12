PROJECT_NAME := Pulumi SDK
SDKS         := dotnet nodejs python go
SUB_PROJECTS := $(SDKS:%=sdk/%)

include build/common.mk

PROJECT         := github.com/pulumi/pulumi/pkg/v3/cmd/pulumi
# To enable excluding longest running tests to run in separate workers
PKG_CODEGEN_NODEJS := github.com/pulumi/pulumi/pkg/v3/codegen/nodejs
PKG_CODEGEN_PYTHON := github.com/pulumi/pulumi/pkg/v3/codegen/python
PKG_CODEGEN_DOTNET := github.com/pulumi/pulumi/pkg/v3/codegen/dotnet
PKG_CODEGEN_GO     := github.com/pulumi/pulumi/pkg/v3/codegen/go
# nodejs and python codegen tests are much slower than go/dotnet:
PROJECT_PKGS    := $(shell cd ./pkg && go list ./... | grep -v -E '^(${PKG_CODEGEN_NODEJS}|${PKG_CODEGEN_PYTHON})$$')
INTEGRATION_PKG := github.com/pulumi/pulumi/tests/integration
TESTS_PKGS      := $(shell cd ./tests && go list -tags all ./... | grep -v tests/templates | grep -v ^${INTEGRATION_PKG}$)
VERSION         := $(shell pulumictl get version)

TESTPARALLELISM ?= 10

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build $(SUB_PROJECTS:%=%_install)

GO_TEST      = $(PYTHON) ../scripts/go-test.py $(GO_TEST_FLAGS)
GO_TEST_FAST = $(PYTHON) ../scripts/go-test.py $(GO_TEST_FAST_FLAGS)

ensure: .ensure.phony pulumictl.ensure go.ensure $(SUB_PROJECTS:%=%_ensure)
.ensure.phony: sdk/go.mod pkg/go.mod tests/go.mod
	cd sdk && go mod download
	cd pkg && go mod download
	cd tests && go mod download
	@touch .ensure.phony

PROTO_FILES := $(sort $(wildcard proto/**/*.proto) proto/generate.sh $(wildcard proto/build-container/**/*))
build-proto:
	@printf "Protobuffer interfaces are ....... "
	@if [ "$$(cat proto/.checksum.txt)" = "$$(cksum $(PROTO_FILES))" ]; then \
		printf "\033[0;32mup to date\033[0m\n"; \
	else \
		printf "\033[0;34mout of date: REBUILDING\033[0m\n"; \
		cd proto && ./generate.sh || exit 1; \
		cd ../ && cksum $(PROTO_FILES) > proto/.checksum.txt; \
		printf "\033[0;34mProtobuffer interfaces have been \033[0;32mREBUILT\033[0m\n"; \
	fi

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "This command does not do anything anymore. It will be removed in a future version."

ifeq ($(PULUMI_TEST_COVERAGE_PATH),)
build:: build-proto go.ensure
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v3/version.Version=${VERSION}" ${PROJECT}

install:: .ensure.phony go.ensure
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
brew::
	./scripts/brew.sh "${PROJECT}"

.PHONY: lint_pkg lint_sdk lint_tests
lint:: golangci-lint.ensure lint_pkg lint_sdk lint_tests
lint_pkg: lint_deps
	cd pkg && golangci-lint run -c ../.golangci.yml --timeout 5m
lint_sdk: lint_deps
	cd sdk && golangci-lint run -c ../.golangci.yml --timeout 5m
lint_tests: lint_deps
	cd tests && golangci-lint run -c ../.golangci.yml --timeout 5m
lint_deps:
	@echo "Check for golangci-lint"; [ -e "$(shell which golangci-lint)" ]

test_fast:: build get_schemas
	@cd pkg && $(GO_TEST_FAST) ${PROJECT_PKGS} ${PKG_CODEGEN_NODE}

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

test_pkg_nodejs: get_schemas
# this is not invoked as part of test_pkg_rest, in order to improve CI velocity by running this
# target in a separate CI job.
	@cd pkg && $(GO_TEST) ${PKG_CODEGEN_NODEJS}

test_pkg_python: get_schemas
# this is not invoked as part of test_pkg_rest, in order to improve CI velocity by running this
# target in a separate CI job.
	@cd pkg && $(GO_TEST) ${PKG_CODEGEN_PYTHON}

test_pkg_dotnet: get_schemas
# invoked as part of "test_pkg_rest", listed separately to update codegen just for dotnet
	@cd pkg && $(GO_TEST) ${PKG_CODEGEN_DOTNET}

test_pkg_go: get_schemas
# invoked as part of "test_pkg_rest", listed separately to update codegen just for go
	@cd pkg && $(GO_TEST) ${PKG_CODEGEN_GO}

test_pkg_rest: get_schemas
	@cd pkg && $(GO_TEST) ${PROJECT_PKGS}

test_pkg:: test_pkg_nodejs test_pkg_python test_pkg_rest

subset=$(subst test_integration_,,$(word 1,$(subst !, ,$@)))
test_integration_%:
	@cd tests && PULUMI_INTEGRATION_TESTS=$(subset) $(GO_TEST) $(INTEGRATION_PKG)

test_integration_subpkgs:
	@cd tests && $(GO_TEST) $(TESTS_PKGS)

test_integration:: $(SDKS:%=test_integration_%) test_integration_rest test_integration_subpkgs

tidy::
	./scripts/tidy.sh

validate_codecov_yaml::
	curl --data-binary @codecov.yml https://codecov.io/validate

# We replace the '!' with a space, then take the first word
# schema-pkg!x.y.z => schema-pkg
# We then replace 'schema-' with nothing, giving only the package name.
# schema-pkg => pkg
# Recall that `$@` is the target make is trying to build, in our case schema-pkg!x.y.z
name=$(subst schema-,,$(word 1,$(subst !, ,$@)))
# Here we take the second word, just the version
version=$(word 2,$(subst !, ,$@))
schema-%: curl.ensure jq.ensure
	@echo "Ensuring schema ${name}, ${version}"
	@# Download the package from github, then stamp in the correct version.
	@[ -f pkg/codegen/testing/test/testdata/${name}.json ] || \
		curl "https://raw.githubusercontent.com/pulumi/pulumi-${name}/v${version}/provider/cmd/pulumi-resource-${name}/schema.json" \
	 	| jq '.version = "${version}"' >  pkg/codegen/testing/test/testdata/${name}.json
	@# Confirm that the correct version is present. If not, error out.
	@FOUND="$$(jq -r '.version' pkg/codegen/testing/test/testdata/${name}.json)" &&        \
		if ! [ "$$FOUND" = "${version}" ]; then									           \
			echo "${name} required version ${version} but found existing version $$FOUND"; \
			exit 1;																		   \
		fi
get_schemas: schema-aws!4.26.0          \
			 schema-azure-native!1.29.0 \
			 schema-azure!4.18.0        \
			 schema-kubernetes!3.7.2    \
			 schema-random!4.2.0        \
			 schema-eks!0.37.1
