PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := pulumi
VERSION          := $(shell git describe --tags --dirty 2>/dev/null)

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

ensure::
	cd runtime/native && ./ensure_node_v8.sh

lint::
	tslint -c tslint.json -p tsconfig.json

build::
	cd runtime/native && node-gyp configure
	cp -R ../proto/nodejs/. proto/
	cd runtime/native/ && node-gyp build
	tsc
	cp package.json bin/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" bin/version.js bin/package.json && \
		rm bin/version.js.bak bin/package.json.bak
	cp -R proto/. bin/proto/
	mkdir -p bin/runtime/native/
	cp -R runtime/native/build/. bin/runtime/native/build/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

install::
	cp pulumi-langhost-nodejs "$(PULUMI_BIN)"
	cp pulumi-provider-pulumi-nodejs "$(PULUMI_BIN)"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/tests"

test_fast::
	istanbul cover --print none _mocha -- --timeout 15000 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text
