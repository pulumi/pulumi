PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION 		 := $(shell cd ../../ && pulumictl get version --language javascript)

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/v3/nodejs/cmd/pulumi-language-nodejs

PROJECT_PKGS    := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

lint::
	./node_modules/.bin/tslint -c tslint.json -p tsconfig.json

build_package::
	./node_modules/.bin/tsc
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

istanbul_tests:: build
	./node_modules/.bin/istanbul test --print none _mocha -- --timeout 120000 'bin/tests/**/*.spec.js'
	./node_modules/.bin/istanbul report text-summary
	./node_modules/.bin/istanbul report text
	./node_modules/.bin/istanbul test --print none _mocha -- 'bin/tests_with_mocks/**/*.spec.js'

sxs_tests:: build
	pushd tests/sxs_ts_3.6 && yarn ; tsc ; popd
	pushd tests/sxs_ts_latest && yarn ; tsc ; popd

test_fast:: sxs_tests istanbul_tests
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: sxs_tests istanbul_tests
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
