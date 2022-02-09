
IMG ?= pulumi_mapper
ACCESS_TOKEN_USR ?= nothing
ACCESS_TOKEN_PWD ?= nothing

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GIT_HASH=$(shell git rev-parse --short HEAD)
GIT_BRANCH=$(shell git symbolic-ref --short HEAD)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

fmt: ## Run go fmt against code.
	go fmt ./...

build: fmt
	go build -o bin/opa-calculator main.go

run: fmt
	go run ./main.go

docker-build: fmt
	docker buildx build  -f Dockerfile --build-arg ACCESS_TOKEN_USR=${ACCESS_TOKEN_USR} --build-arg ACCESS_TOKEN_PWD=${ACCESS_TOKEN_PWD} -t ${IMG}:latest .

tag-version:
	@echo 'create tag $(GIT_HASH)'
	docker tag ${IMG}:latest ${IMG}:${GIT_HASH}

publish: publish-latest publish-version

publish-latest:
	@echo 'publish latest'
	docker push ${IMG}:latest

publish-version: tag-version
	@echo 'publish $(VERSION)'
	docker push ${IMG}:${GIT_HASH}

