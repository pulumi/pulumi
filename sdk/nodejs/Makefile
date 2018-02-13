PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION          := $(shell git describe --tags --dirty 2>/dev/null)

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-langhost-nodejs

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

PROJECT_PKGS    := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

ensure::
	cd runtime/native && ./ensure_node_v8.sh

lint::
	$(GOMETALINTER) cmd/pulumi-langhost-nodejs/main.go | sort ; exit "$${PIPESTATUS[0]}"
	tslint -c tslint.json -p tsconfig.json

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cd runtime/native && node-gyp configure
	cp -R ../proto/nodejs/. proto/
	cd runtime/native/ && node-gyp build
	tsc
	cp README.md ../../LICENSE package.json bin/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" bin/version.js bin/package.json && \
		rm bin/version.js.bak bin/package.json.bak
	cp -R proto/. bin/proto/
	mkdir -p bin/runtime/native/
	cp -R runtime/native/build/. bin/runtime/native/build/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp pulumi-langhost-nodejs-exec "$(PULUMI_BIN)"
	cp pulumi-provider-pulumi-nodejs "$(PULUMI_BIN)"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/tests"

test_fast::
	istanbul cover --print none _mocha -- --timeout 15000 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text
	go test -timeout $(TEST_FAST_TIMEOUT) -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}
