PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION          := $(shell ../../scripts/get-version)

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs

PROJECT_PKGS    := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

lint::
	golangci-lint run
	./node_modules/.bin/tslint -c tslint.json -p tsconfig.json

build_package::
	./node_modules/.bin/tsc
	cp tests/runtime/jsClosureCases.js bin/tests/runtime
	cp README.md ../../LICENSE package.json ./dist/* bin/
	node ../../scripts/reversion.js bin/package.json ${VERSION}
	node ../../scripts/reversion.js bin/version.js ${VERSION}
	cp -R proto/. bin/proto/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}

build:: build_package build_plugin

install_package::
	cp dist/pulumi-resource-pulumi-nodejs "$(PULUMI_BIN)"
	mkdir -p "$(PULUMI_BIN)/v6.10.2"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/tests"

install_plugin::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}

install:: install_package install_plugin

instanbul_tests::
	istanbul test --print none _mocha -- --timeout 15000 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text

test_fast:: instanbul_tests
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all:: instanbul_tests
	$(GO_TEST) ${PROJECT_PKGS}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$$(go env GOPATH)"/bin/
