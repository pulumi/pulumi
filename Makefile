PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/dotnet sdk/nodejs sdk/python sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi
PROJECT_PKGS    := $(shell go list ./pkg/... | grep -v /vendor/)
EXTRA_TEST_PKGS := $(shell go list ./examples/ ./tests/... | grep -v tests/templates | grep -v /vendor/)
VERSION         := $(shell scripts/get-version HEAD)

TESTPARALLELISM := 10

ensure::
	$(call STEP_MESSAGE)
ifeq ($(NOPROXY), true)
	@echo "cd sdk && GO111MODULE=on go mod tidy"; cd sdk && GO111MODULE=on go mod tidy
	@echo "cd sdk && GO111MODULE=on go mod vendor"; cd sdk && GO111MODULE=on go mod vendor
	@echo "cd pkg && GO111MODULE=on go mod tidy"; cd pkg && GO111MODULE=on go mod tidy
	@echo "cd pkg && GO111MODULE=on go mod vendor"; cd pkg && GO111MODULE=on go mod vendor
	@echo "cd examples && GO111MODULE=on go mod tidy"; cd examples && GO111MODULE=on go mod tidy
	@echo "cd examples && GO111MODULE=on go mod vendor"; cd examples && GO111MODULE=on go mod vendor
	@echo "cd tests && GO111MODULE=on go mod tidy"; cd tests && GO111MODULE=on go mod tidy
	@echo "cd tests && GO111MODULE=on go mod vendor"; cd tests && GO111MODULE=on go mod vendor
else
	@echo "cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor"; cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor
	@echo "cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor"; cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor
	@echo "cd examples && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd examples && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd examples && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor"; cd examples && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor
	@echo "cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor"; cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod vendor
endif


build-proto::
	cd sdk/proto && ./generate.sh

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "Generate static assets bundle for docs generator"
	go generate ./pkg/codegen/docs/

build::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

install::
	GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/pkg/version.Version=${VERSION}" ${PROJECT}

lint::
	for DIR in "examples" "pkg" "sdk" "tests" ; do \
		pushd $$DIR && golangci-lint run -c ../.golangci.yml --deadline 5m && popd ; \
	done

test_fast::
	$(GO_TEST_FAST) ${PROJECT_PKGS}

test_all::
	$(GO_TEST) ${PROJECT_PKGS}
	$(GO_TEST) -v -p=1 ${EXTRA_TEST_PKGS}

.PHONY: publish_tgz
publish_tgz:
	$(call STEP_MESSAGE)
	./scripts/publish_tgz.sh

.PHONY: publish_packages
publish_packages:
	$(call STEP_MESSAGE)
	./scripts/publish_packages.sh

.PHONY: coverage
coverage:
	$(call STEP_MESSAGE)
	./scripts/gocover.sh

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all coverage
travis_push: only_build publish_tgz only_test publish_packages
travis_pull_request: all
travis_api: all
