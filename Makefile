.PHONY: banner build install lint lint_quiet test vet

PROJECT=github.com/pulumi/lumi
PROJECT_PKGS=$(shell go list ./... | grep -v /vendor/)
CORE_PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/...)
PROCCNT=$(shell nproc --all)

default: banner test_core lint_quiet vet install
full: banner_full test lint_quiet vet install

banner:
	@echo "\033[0;33mRunning quick build; to run full tests, run 'make full'\033[0m"
	@echo "\033[0;33mRemember to do this before checkin, otherwise your CI will fail\033[0m"
	@go version

banner_full:
	@go version

build:
	@echo "\033[0;32mBUILD:\033[0m"
	@go build ${PROJECT}/cmd/lumi
	@go build ${PROJECT}/cmd/lumidl

install:
	@echo "\033[0;32mINSTALL:\033[0m"
	@go install ${PROJECT}/cmd/lumi
	@go install ${PROJECT}/cmd/lumidl

lint:
	@echo "\033[0;32mLINT:\033[0m"
	@golint -set_exit_status cmd/...
	@golint -set_exit_status pkg/...

lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	@echo "`golint cmd/... | grep -v "or be unexported"`"
	@echo "`golint pkg/... | grep -v "or be unexported"`"
	test -z "$$(golint cmd/... | grep -v 'or be unexported')"
	test -z "$$(golint pkg/... | grep -v 'or be unexported')"
	@echo "\033[0;33mgolint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

test:
	@echo "\033[0;32mTEST:\033[0m"
	@go test -parallel ${PROCCNT} -cover ${PROJECT_PKGS}

test_core:
	@echo "\033[0;32mTEST (core):\033[0m"
	@go test -parallel ${PROCCNT} -cover ${CORE_PROJECT_PKGS}

vet:
	@echo "\033[0;32mVET:\033[0m"
	@go tool vet -printf=false cmd/ lib/ pkg/

