PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION 		 := $(shell cd ../../ && pulumictl get version --language javascript)

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/v3/nodejs/cmd/pulumi-language-nodejs
PROJECT_ROOT     := $(realpath ../..)

PROJECT_PKGS    := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS = build

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

lint::
	eslint -c .eslintrc.js --ext .ts .

build_package::
	yarn run tsc
	cp tests/runtime/jsClosureCases_8.js bin/tests/runtime
	cp tests/runtime/jsClosureCases_10_4.js bin/tests/runtime
	cp -R tests/automation/data/. bin/tests/automation/data/
	cp README.md ../../LICENSE package.json ./dist/* bin/
	node ../../scripts/reversion.js bin/package.json ${VERSION}
	node ../../scripts/reversion.js bin/version.js ${VERSION}
	cp -R proto/. bin/proto/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}

build:: build_package build_plugin

install_package:: build
	cp dist/pulumi-resource-pulumi-nodejs "$(PULUMI_BIN)"
	cp dist/pulumi-analyzer-policy "$(PULUMI_BIN)"

install_plugin:: build
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}

install:: install_package install_plugin

istanbul_tests:: $(TEST_ALL_DEPS)
	$(TESTSUITE_SKIPPED) istanbul || istanbul test --print none _mocha -- --timeout 120000 --exclude 'bin/tests/automation/**/*.spec.js' 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text
	$(TESTSUITE_SKIPPED) istanbul-with-mocks || istanbul test --print none _mocha -- 'bin/tests_with_mocks/**/*.spec.js'

auto_tests:: $(TEST_ALL_DEPS)
	$(TESTSUITE_SKIPPED) auto-nodejs || istanbul test --print none _mocha -- --timeout 120000 'bin/tests/automation/**/*.spec.js'
	istanbul report text-summary
	istanbul report text

sxs_tests:: $(TEST_ALL_DEPS)
	echo "SKIP tests/sxs_ts_3.6" # pushd tests/sxs_ts_3.6 && yarn ; tsc ; popd
	echo "SKIP tests/sxs_ts_latest" # pushd tests/sxs_ts_latest && yarn ; tsc ; popd

test_fast:: sxs_tests istanbul_tests
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: sxs_tests istanbul_tests auto_tests
	$(GO_TEST) ${PROJECT_PKGS}

dist:: build
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
	cp dist/pulumi-analyzer-policy "$$(go env GOPATH)"/bin/

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
	cp dist/pulumi-analyzer-policy "$$(go env GOPATH)"/bin/

publish:: build_package
	bash -c ../../scripts/publish_npm.sh
