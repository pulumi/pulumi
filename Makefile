.PHONY: build install lint nolint test vet

PROJECT=github.com/pulumi/coconut
PROJECT_PKGS=$(shell go list ./... | grep -v /vendor/)

default: test nolint vet install

build:
	@echo "\033[0;32mBUILD:\033[0m"
	@go build ${PROJECT}

install:
	@echo "\033[0;32mINSTALL:\033[0m"
	@go install ${PROJECT}

lint:
	@echo "\033[0;32mLINT:\033[0m"
	@golint cmd/...
	@golint pkg/...

nolint:
	@echo "\033[0;32mLINT:\033[0m"
	@echo "\033[0;33mgolint not run on build automatically; to run, make lint\033[0m"

test:
	@echo "\033[0;32mTEST:\033[0m"
	@go test ${PROJECT_PKGS}

vet:
	@echo "\033[0;32mVET:\033[0m"
	@go vet ${PROJECT_PKGS}

