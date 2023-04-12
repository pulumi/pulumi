PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh javascript))

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/v3/nodejs/cmd/pulumi-language-nodejs

PROJECT_PKGS    := $(shell go list ./cmd...)
TEST_FAST_TIMEOUT := 2m

ifeq ($(DEBUG),"true")
$(info    VERSION         = $(VERSION))
endif

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

GO_TEST      = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FLAGS)
GO_TEST_FAST = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FAST_FLAGS)

ensure:: yarn.ensure node.ensure .ensure.phony
.ensure.phony: package.json
	yarn install --frozen-lockfile
	@touch .ensure.phony

lint:: ensure
	yarn run eslint -c .eslintrc.js --ext .ts .

build_package:: ensure
	yarn run tsc
	cp tests/runtime/jsClosureCases_8.js bin/tests/runtime
	cp tests/runtime/jsClosureCases_10_4.js bin/tests/runtime
	mkdir -p bin/tests/automation/data/
	cp -R tests/automation/data/. bin/tests/automation/data/
	cp README.md ../../LICENSE ./dist/* bin/
# safely remove mockpackage dependency see [pulumi/pulumi#9026]
	cp package.json yarn.lock bin/ && cd bin && yarn remove mockpackage && rm yarn.lock
	node ../../scripts/reversion.js bin/package.json ${VERSION}
	node ../../scripts/reversion.js bin/version.js ${VERSION}
	mkdir -p bin/proto
	cp -R proto/. bin/proto/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}

build:: build_package build_plugin

install_package:: build
	cp dist/pulumi-resource-pulumi-nodejs* "$(PULUMI_BIN)"
	cp dist/pulumi-analyzer-policy* "$(PULUMI_BIN)"

install_plugin:: build
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}

install:: install_package install_plugin

unit_tests:: $(TEST_ALL_DEPS)
	yarn run nyc -x 'bin/tests/runtime/*ClosureCases*.js' -s mocha --timeout 120000 \
		--exclude 'bin/tests/automation/**/*.spec.js' \
		--exclude 'bin/tests/runtime/closureLoader.spec.js' \
		'bin/tests/**/*.spec.js'
	yarn run mocha --timeout 120000 'bin/tests/runtime/closureLoader.spec.js'
	yarn run nyc -s mocha 'bin/tests_with_mocks/**/*.spec.js'

test_auto:: $(TEST_ALL_DEPS)
	yarn run nyc -s mocha --timeout 220000 'bin/tests/automation/**/*.spec.js'

TSC_SUPPORTED_VERSIONS = ~3.7.3 ^3 ^4

version=$(subst sxs_test_,,$(word 1,$(subst !, ,$@)))
sxs_test_%:
	@cd tests/sxs_ts_test && ( \
		yarn add typescript@$(version); \
		yarn run tsc; \
		echo "✅ TypeScript $(version) passed"; \
		git checkout package.json; \
	) || ( \
		echo "🚫 TypeScript $(version) failed"; \
		git checkout package.json; \
		exit 1; \
	)

sxs_tests:: $(TEST_ALL_DEPS) $(TSC_SUPPORTED_VERSIONS:%=sxs_test_%)

test_fast:: sxs_tests unit_tests
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .nyc_output ]; then yarn run nyc report -r cobertura --report-dir $(PULUMI_TEST_COVERAGE_PATH); fi
endif
	@$(GO_TEST_FAST) ${PROJECT_PKGS}

test_go:: $(TEST_ALL_DEPS)
	@$(GO_TEST) ${PROJECT_PKGS}

test_all:: sxs_tests unit_tests test_auto test_go
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .nyc_output ]; then yarn run nyc report -r cobertura --report-dir $(PULUMI_TEST_COVERAGE_PATH); fi
endif

dist:: build
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
	cp dist/pulumi-analyzer-policy "$$(go env GOPATH)"/bin/

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
	cp dist/pulumi-analyzer-policy "$$(go env GOPATH)"/bin/

publish::
	bash -c ../../scripts/publish_npm.sh
