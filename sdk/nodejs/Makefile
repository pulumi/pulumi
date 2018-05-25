PROJECT_NAME     := Pulumi Node.JS SDK
NODE_MODULE_NAME := @pulumi/pulumi
VERSION          := $(shell ../../scripts/get-version)

LANGUAGE_HOST    := github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

PROJECT_PKGS    := $(shell go list ./cmd...)
TESTPARALLELISM := 10
TEST_FAST_TIMEOUT := 2m

include ../../build/common.mk

export PATH:=$(shell yarn bin 2>/dev/null):$(PATH)

ensure::
	[ -d "./prebuilt/" ] || mkdir "./prebuilt"
	[ -e "./prebuilt/nativeruntime.node" ] || aws s3 cp "s3://eng.pulumi.com/nativeruntime/$$(go env GOOS)/nativeruntime.node" "./prebuilt/"
	[ -e "./prebuilt/nativeruntime-v0.11.0.node" ] || aws s3 cp "s3://eng.pulumi.com/nativeruntime/$$(go env GOOS)/nativeruntime-v0.11.0.node" "./prebuilt/"

lint::
	$(GOMETALINTER) cmd/pulumi-language-nodejs/main.go | sort ; exit "$${PIPESTATUS[0]}"
	tslint -c tslint.json -p tsconfig.json

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}
	tsc
	cp tests/runtime/jsClosureCases.js bin/tests/runtime
	cp README.md ../../LICENSE package.json ./dist/* bin/
	node ../../scripts/reversion.js bin/package.json ${VERSION}
	node ../../scripts/reversion.js bin/version.js ${VERSION}
	cp -R proto/. bin/proto/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

docs::
	./node_modules/.bin/typedoc --json bin/docs.json --mode modules --includeDeclarations --excludeExternals --excludeNotExported

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${LANGUAGE_HOST}
	cp dist/pulumi-resource-pulumi-nodejs "$(PULUMI_BIN)"
	mkdir -p "$(PULUMI_BIN)/v6.10.2"
	cp ./prebuilt/*.node "$(PULUMI_BIN)/v6.10.2/"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/tests"

test_fast::
	istanbul test --print none _mocha -- --timeout 15000 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text
	go test -timeout $(TEST_FAST_TIMEOUT) -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}
