PROJECT_NAME := Pulumi npm CLI wrapper
VERSION      := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../scripts/pulumi-version.sh javascript))

include ../build/common.mk

ensure:: .make/ensure/npm
.make/ensure/npm: package.json
	npm install --ignore-scripts
	@mkdir -p .make/ensure && touch .make/ensure/npm

format:: ensure
	npx @biomejs/biome format --write .

lint:: ensure
	npx @biomejs/biome ci .

build:: ensure

test:: ensure
	npm test
	npm run test:integration

publish:: build
	NPM_PKG_JSON=$(CURDIR)/package.json NPM_ARTIFACT_GLOB="sdk-npm-*.tgz" bash -c ../scripts/publish_npm.sh
