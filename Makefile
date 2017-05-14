.PHONY: build install lint lint_quiet test vet

PROJECT=github.com/pulumi/coconut
PROJECT_PKGS=$(shell go list ./... | grep -v /vendor/)

default: test lint_quiet vet install

build:
	@echo "\033[0;32mBUILD:\033[0m"
	@go build ${PROJECT}/cmd/coco

install:
	@echo "\033[0;32mINSTALL:\033[0m"
	@go install ${PROJECT}/cmd/coco

lint:
	@echo "\033[0;32mLINT:\033[0m"
	@golint cmd/...
	@golint pkg/...

lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	@echo "`golint cmd/... | grep -v "or be unexported"`"
	@echo "`golint pkg/... | grep -v "or be unexported"`"
	@echo "\033[0;33mgolint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

test:
	@echo "\033[0;32mTEST:\033[0m"
	@go test -cover ${PROJECT_PKGS}

vet:
	@echo "\033[0;32mVET:\033[0m"
	@go vet ${PROJECT_PKGS}

