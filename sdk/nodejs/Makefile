PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh javascript))

LANGHOST_PKG    := github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3

TEST_FAST_TIMEOUT := 2m

ifeq ($(DEBUG),"true")
$(info    VERSION         = $(VERSION))
endif

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

ensure:: .make/ensure/yarn .make/ensure/node .make/ensure/phony
.make/ensure/phony: package.json
	../../scripts/retry yarn install --frozen-lockfile
	@mkdir -p .make/ensure && touch .make/ensure/phony

format:: ensure
	yarn biome format --write .

lint:: ensure
	yarn run eslint -c .eslintrc.js --ext .ts .
	yarn biome ci .

lint_fix:: ensure
	yarn run eslint -c .eslintrc.js --ext .ts --fix .
	yarn biome format --write .

build_package:: ensure
	yarn run tsc
	mkdir -p bin/tests/automation/data/
	cp -R tests/automation/data/. bin/tests/automation/data/
	cp -R tests/provider/experimental/testdata/ bin/tests/provider/experimental/testdata
	cp .npmignore README.md ../../LICENSE ./dist/* bin/
	cp -R ./vendor/ ./bin/vendor/
	node ../../scripts/reversion.js bin/package.json ${VERSION}
	node ../../scripts/reversion.js bin/version.js ${VERSION}
	mkdir -p bin/proto
	cp -R proto/. bin/proto/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

build_plugin::
	go build -C cmd/pulumi-language-nodejs \
    	-o ../../../../bin/pulumi-language-nodejs \
    	-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
        ${LANGHOST_PKG}

build:: build_package build_plugin

install_package:: build
	cp dist/pulumi-resource-pulumi-nodejs* "$(PULUMI_BIN)"
	cp dist/pulumi-analyzer-policy* "$(PULUMI_BIN)"

install_plugin:: build
	GOBIN=$(PULUMI_BIN) go install -C cmd/pulumi-language-nodejs \
	   -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_package install_plugin

unit_tests:: $(TEST_ALL_DEPS)
	yarn run nyc --no-clean -s mocha --timeout 120000 \
		--exclude 'bin/tests/automation/**/*.spec.js' \
		--exclude 'bin/tests/runtime/closure-integration-tests.js' \
		'bin/tests/**/*.spec.js'
	yarn run nyc --no-clean -s mocha 'bin/tests_with_mocks/**/*.spec.js'

test_auto:: $(TEST_ALL_DEPS)
	yarn run nyc --no-clean -s mocha --timeout 300000 'bin/tests/automation/**/*.spec.js'

test_integration:: $(TEST_ALL_DEPS)
	node 'bin/tests/runtime/closure-integration-tests.js'
	node 'bin/tests/runtime/install-package-tests.js'

TSC_SUPPORTED_VERSIONS = ~3.8.3 ^3 ^4

version=$(subst sxs_test_,,$(word 1,$(subst !, ,$@)))
sxs_test_%:
	@cd tests/sxs_ts_test && ( \
		cp -f package$(version).json package.json && \
		yarn install && \
		yarn run tsc --version && \
		yarn run tsc &&  \
		rm package.json && \
		echo "✅ TypeScript $(version) passed" \
	) || ( \
		echo "🚫 TypeScript $(version) failed"; \
		exit 1; \
	)

sxs_tests:: $(TSC_SUPPORTED_VERSIONS:%=sxs_test_%)

test_fast:: sxs_tests unit_tests
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .nyc_output ]; then yarn run nyc report -r cobertura --report-dir $(PULUMI_TEST_COVERAGE_PATH); fi
endif
	@cd cmd/pulumi-language-nodejs && $(GO_TEST_FAST) $(shell go list ./... | grep -v /vendor/ | grep -v templates)
	$(GO_TEST_FAST) $(shell go list ./...)

test_go:: $(TEST_ALL_DEPS)
	@cd cmd/pulumi-language-nodejs && $(GO_TEST) $(shell go list ./... | grep -v /vendor/ | grep -v templates)
	$(GO_TEST) $(shell go list ./...)

test_all:: sxs_tests unit_tests test_auto test_go test_integration
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .nyc_output ]; then yarn run nyc report -r cobertura --report-dir $(PULUMI_TEST_COVERAGE_PATH); fi
endif

PULUMI_TEST_ORG ?= $(shell pulumi whoami --json | jq ".organizations[0]")
TEST_GREP ?= .*

# Watches all the typescript files and runs the tests every time they change.
# Can optionally be given a regex of tests to match. For example, to iterate
# on one test specifically, one could use the following command:
#
# $ make test_watch TEST_GREP="lists tag values"
test_watch::
	PULUMI_TEST_ORG=$(PULUMI_TEST_ORG) yarn mocha "**/*.spec.ts" --timeout 300000 --bail -j 1 --watch --watch-files "**/*.ts" --grep "$(TEST_GREP)"

dist:: GOBIN=$(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
dist::
	go install -C cmd/pulumi-language-nodejs \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp dist/pulumi-resource-pulumi-nodejs "${GOBIN}"
	cp dist/pulumi-analyzer-policy "${GOBIN}"

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -C cmd/pulumi-language-nodejs \
	   -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
	cp dist/pulumi-analyzer-policy "$$(go env GOPATH)"/bin/

publish::
	bash -c ../../scripts/publish_npm.sh
