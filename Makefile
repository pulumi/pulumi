PROJECT=github.com/pulumi/lumi
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
PROCCNT=$(shell nproc --all)

.PHONY: default
default: banner lint_quiet vet test install

.PHONY: all
all: banner_all lint_quiet vet test install lumijs aws_pkg

.PHONY: banner
banner:
	@echo "\033[1;37m============\033[0m"
	@echo "\033[1;37mLumi (Quick)\033[0m"
	@echo "\033[1;37m============\033[0m"
	@echo "\033[0;33mRunning quick build; to run full tests, run 'make all'\033[0m"
	@echo "\033[0;33mRemember to do this before checkin, otherwise your CI will fail\033[0m"

.PHONY: banner_all
banner_all:
	@echo "\033[1;37m============\033[0m"
	@echo "\033[1;37mLumi (Full)\033[0m"
	@echo "\033[1;37m============\033[0m"

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	@go version
	@go build ${PROJECT}/cmd/lumi
	@go build ${PROJECT}/cmd/lumidl

.PHONY: full_build
full_build: build lumijs_build aws_pkg_build

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m"
	@go install ${PROJECT}/cmd/lumi
	@go install ${PROJECT}/cmd/lumidl

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	@golint -set_exit_status cmd/...
	@golint -set_exit_status pkg/...

.PHONY: lint_quiet
lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	@echo "`golint cmd/... | grep -v "or be unexported"`"
	@echo "`golint pkg/... | grep -v "or be unexported"`"
	@test -z "$$(golint cmd/... | grep -v 'or be unexported')"
	@test -z "$$(golint pkg/... | grep -v 'or be unexported')"
	@echo "\033[0;33mgolint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

.PHONY: vet
vet:
	@echo "\033[0;32mVET:\033[0m"
	@go tool vet -printf=false cmd/ pkg/

.PHONY: test
test:
	@echo "\033[0;32mTEST:\033[0m"
	@go test -parallel ${PROCCNT} -cover ${PROJECT_PKGS}

.PHONY: lumijs
lumijs:
	@cd ./cmd/lumijs && $(MAKE)

.PHONY: aws_pkg
aws_pkg:
	@cd ./lib/aws && $(MAKE)

.PHONY: verify
verify:
	@cd ./lib/aws && $(MAKE) verify

