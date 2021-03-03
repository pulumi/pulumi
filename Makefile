PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/dotnet sdk/nodejs sdk/python sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi/pkg/v2/cmd/pulumi
PROJECT_PKGS    := $(shell cd ./pkg && go list ./... | grep -v /vendor/)
TESTS_PKGS      := $(shell cd ./tests && go list ./... | grep -v tests/templates | grep -v /vendor/)
VERSION         := $(shell pulumictl get version)

TESTPARALLELISM := 10

ensure::
	$(call STEP_MESSAGE)
ifeq ($(NOPROXY), true)
	@echo "cd sdk && GO111MODULE=on go mod tidy"; cd sdk && GO111MODULE=on go mod tidy
	@echo "cd sdk && GO111MODULE=on go mod download"; cd sdk && GO111MODULE=on go mod download
	@echo "cd pkg && GO111MODULE=on go mod tidy"; cd pkg && GO111MODULE=on go mod tidy
	@echo "cd pkg && GO111MODULE=on go mod download"; cd pkg && GO111MODULE=on go mod download
	@echo "cd scripts && GO111MODULE=on go mod tidy"; cd scripts && GO111MODULE=on go mod tidy
	@echo "cd scripts && GO111MODULE=on go mod download"; cd scripts && GO111MODULE=on go mod download
	@echo "cd tests && GO111MODULE=on go mod tidy"; cd tests && GO111MODULE=on go mod tidy
	@echo "cd tests && GO111MODULE=on go mod download"; cd tests && GO111MODULE=on go mod download
	@echo "cd scripts && GO111MODULE=on go mod tidy"; cd scripts && GO111MODULE=on go mod tidy
	@echo "cd scripts && GO111MODULE=on go mod download"; cd scripts && GO111MODULE=on go mod download
else
	@echo "cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download"; cd sdk && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download
	@echo "cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download"; cd pkg && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download
	@echo "cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download"; cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download
	@echo "cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download"; cd tests && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download
	@echo "cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy"; cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod tidy
	@echo "cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download"; cd scripts && GO111MODULE=on GOPROXY=$(GOPROXY) go mod download
endif


build-proto::
	cd sdk/proto && ./generate.sh

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "Generate static assets bundle for docs generator"
	cd pkg && go generate ./codegen/docs/gen.go

build:: generate
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

build_debug:: generate
	cd pkg && go install -gcflags="all=-N -l" -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

install:: generate
	cd pkg && GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

install_all:: install

dist:: build
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

# NOTE: the brew target intentionally avoids the dependency on `build`, as it does not require the language SDKs.
brew::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

lint::
	for DIR in "pkg" "sdk" "tests" ; do \
		pushd $$DIR ; golangci-lint run -c ../.golangci.yml --timeout 5m ; popd ; \
	done

test_fast:: build
	cd pkg && $(GO_TEST_FAST) ${PROJECT_PKGS}

test_build:: $(SUB_PROJECTS:%=%_install)
	cd tests/integration/construct_component/testcomponent && yarn install && yarn link @pulumi/pulumi && yarn run tsc

test_all:: build test_build $(SUB_PROJECTS:%=%_install)
	cd pkg && $(GO_TEST) ${PROJECT_PKGS}
	cd tests && $(GO_TEST) -p=1 ${TESTS_PKGS}

.PHONY: publish_tgz
publish_tgz:
	$(call STEP_MESSAGE)
	./scripts/publish_tgz.sh

.PHONY: publish_packages
publish_packages:
	$(call STEP_MESSAGE)
	./scripts/publish_packages.sh

.PHONY: test_containers
test_containers:
	$(call STEP_MESSAGE)
	./scripts/test-containers.sh ${VERSION}

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: install dist all
travis_push: install dist publish_tgz only_test publish_packages
travis_pull_request:
	$(call STEP_MESSAGE)
	@echo moved to GitHub Actions
travis_api: install dist all
