GOPKGS          = $(shell go list ./provider/... | grep -v /vendor/)
LUMIROOT       ?= /usr/local/lumi
LUMILIB         = ${LUMIROOT}/packs
THISLIB         = ${LUMILIB}/aws
TESTPARALLELISM = 10

.PHONY: default
default: banner lint_quiet vet build test install

.PHONY: banner
banner:
	@echo "\033[1;37m================\033[0m"
	@echo "\033[1;37mLumi AWS Package\033[0m"
	@echo "\033[1;37m================\033[0m"

.PHONY: gen
gen:
	@echo "\033[0;32mGEN:\033[0m"
	lumidl \
	    aws idl/ \
		--recursive \
		--out-pack=pack/ \
		--out-rpc=rpc/

.PHONY: clean
clean:
	rm -rf ./bin
	rm -rf ${THISLIB}

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	cd pack/ && yarn link @lumi/lumirt @lumi/lumijs @lumi/lumi # ensure we resolve to Lumi's stdlib.
	cd pack/ && lumijs # compile the LumiPack
	cd pack/ && lumi pack verify # ensure the pack verifies
	cp -R pack/.lumi/bin/ bin/ # copy the pack to our bin dir
	go version
	cd provider/ && go build -i -o ../bin/lumi-resource-aws # compile the resource provider

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m [${LUMILIB}]"
	cd pack/ && yarn link  # ensure NPM references resolve locally.
	mkdir -p ${LUMILIB} # ensure the machine-wide library dir exists.
	cp -R ./bin/ ${THISLIB} # copy to the standard library location.

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	./pack/node_modules/.bin/tslint './pack/**/*.ts' --exclude='./pack/node_modules/**'
	gometalinter provider/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/lumi#191]: will fix when we write all of our API docs
#    - "cyclomatic complexity": TODO[pulumi/lumi#259]: need to fix a bunch of cyclomatically complex functions.
LINT_SUPPRESS="or be unexported|cyclomatic complexity"

.PHONY: lint_quiet
lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	./pack/node_modules/.bin/tslint './pack/**/*.ts' --exclude='./pack/node_modules/**'
	gometalinter provider/... | grep -vE ${LINT_SUPPRESS} | sort
	@test -z "$$(gometalinter provider/... | grep -vE ${LINT_SUPPRESS})"
	@echo "\033[0;33mgolint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

.PHONY: vet
vet:
	@echo "\033[0;32mVET:\033[0m"
	go tool vet -printf=false provider/

.PHONY: test
test:
	@echo "\033[0;32mTEST:\033[0m"
	go test -cover -parallel ${TESTPARALLELISM} ${GOPKGS}

.PHONY: verify
verify: gen
	$(shell git diff --quiet .)

