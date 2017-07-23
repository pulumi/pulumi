PROCCNT=$(shell nproc --all)
LUMIROOT ?= /usr/local/lumi
LUMILIB   = ${LUMIROOT}/packs
THISLIB   = ${LUMILIB}/lumi

.PHONY: default
default: banner build install

.PHONY: banner
banner:
	@echo "\033[1;37m===================\033[0m"
	@echo "\033[1;37mLumi Stdlib Package\033[0m"
	@echo "\033[1;37m===================\033[0m"

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	@./pack/node_modules/.bin/tslint lib/aws/lumi/...

.PHONY: clean
clean:
	rm -rf ./.lumi/bin
	rm -rf ${THISLIB}

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	@lumijs # compile the LumiPack
	@lumi pack verify # ensure the pack verifies

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m [${LUMILIB}]"
	@yarn link # ensure NPM references resolve locally
	@mkdir -p ${LUMILIB} # ensure the machine-wide library dir exists.
	@cp -R ./.lumi/bin/ ${THISLIB} # copy to the standard library location.
	@cp package.json ${THISLIB} # ensure the result is an NPM package.
	@if [ -d ./node_modules ]; then cp -r ./node_modules ${LUMIROOT}; fi # keep the links we depend upon.

