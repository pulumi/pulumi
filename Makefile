PROJECT_NAME := Pulumi SDK
SUB_PROJECTS := sdk/dotnet sdk/nodejs sdk/python sdk/go
include build/common.mk

PROJECT         := github.com/pulumi/pulumi/pkg/v2/cmd/pulumi
PROJECT_PKGS    := $(shell cd ./pkg && go list ./... | grep -v /vendor/)
TESTS_PKGS      := $(shell cd ./tests && go list ./... | grep -v tests/templates | grep -v /vendor/)
VERSION         := $(shell scripts/get-version HEAD)

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

install:: generate
	cd pkg && GOBIN=$(PULUMI_BIN) go install -ldflags "-X github.com/pulumi/pulumi/pkg/v2/version.Version=${VERSION}" ${PROJECT}

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

test_integration:: install
	cd tests && $(GO_TEST) -v -p=6 ${TESTS_PKGS}

test_all:: build $(SUB_PROJECTS:%=%_install) test_fast test_integration

.PHONY: publish_tgz
publish_tgz:
	$(call STEP_MESSAGE)
	./scripts/publish_tgz.sh

.PHONY: publish_packages
publish_packages:
	$(call STEP_MESSAGE)
	./scripts/publish_packages.sh

pull_request:
	$(call STEP_MESSAGE)

# Run the integration tests for our DockerHub containers. We do so only via the
# "Travis Cron" job type, because (1) the tests can only be ran _after_ we publish
# the current SDK version, since it is required by the Docker build. And (2) the
# tests (currently) aren't reliable enough to run as part of every push to master.
#
# So instead we run the ~daily on master. Where we know the current SDK version
# will have been published.
.PHONY: test_containers_cron
test_containers_cron:
	$(call STEP_MESSAGE)
	./scripts/build-docker.sh ${VERSION} --test

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron travis_push travis_pull_request travis_api
travis_cron: all
travis_push: only_build publish_tgz only_test publish_packages
travis_pull_request: pull_request
travis_api: all
