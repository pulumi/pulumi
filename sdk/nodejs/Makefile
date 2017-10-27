SHELL=/bin/bash
.SHELLFLAGS=-ec
ECHO=echo -e

export PATH:=$(shell yarn bin):$(PATH)

.PHONY: default
default: banner ensure configure build lint test install

.PHONY: banner
banner:
	@$(ECHO) "\033[1;37m==================\033[0m"
	@$(ECHO) "\033[1;37mPulumi Node.js SDK\033[0m"
	@$(ECHO) "\033[1;37m==================\033[0m"

.PHONY: ensure
ensure:
	yarn install

.PHONY: configure
configure:
	cd runtime/native && ./ensure_node_v8.sh
	cd runtime/native && node-gyp configure

.PHONY: lint
lint:
	@$(ECHO) "\033[0;32mLINT:\033[0m"
	tslint -c tslint.json -p tsconfig.json

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: build
build:
	@$(ECHO) "\033[0;32mBUILD:\033[0m"
	cp -R ../proto/nodejs/. proto/
	cd runtime/native/ && node-gyp build
	tsc
	cp -R proto/. bin/proto/
	mkdir -p bin/runtime/native/
	cp -R runtime/native/build/. bin/runtime/native/build/
	mkdir -p bin/tests/runtime/langhost/cases/
	find tests/runtime/langhost/cases/* -type d -exec cp -R {} bin/tests/runtime/langhost/cases/ \;

.PHONY: test
test:
	@$(ECHO) "\033[0;32mTEST:\033[0m"
	istanbul cover --print none _mocha -- --timeout 15000 'bin/tests/**/*.spec.js'
	istanbul report text-summary
	istanbul report text

.PHONY: install
install:
	@$(ECHO) "\033[0;32mINSTALL:\033[0m"
	cp package.json bin/
	cd bin/ && (yarn unlink > /dev/null 2>&1 || true) && yarn link

