SHELL=/bin/bash
.SHELLFLAGS=-ec

PROJECT=github.com/pulumi/pulumi
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
TESTPARALLELISM=10

ECHO=echo -e
GOMETALINTERBIN=gometalinter
GOMETALINTER=${GOMETALINTERBIN} --config=Gometalinter.json
VERSION=$(shell git describe --tags 2>/dev/null)

.PHONY: all
all: banner_all core sdk/nodejs integrationtest

.PHONY: nightly
nightly: all gocover

.PHONY: core
core: vet test install lint_quiet

.PHONY: banner
banner:
	@$(ECHO) "\033[1;37m=====================\033[0m"
	@$(ECHO) "\033[1;37mPulumi Fabric (Quick)\033[0m"
	@$(ECHO) "\033[1;37m=====================\033[0m"
	@$(ECHO) "\033[0;33mRunning quick build; to run full tests, run 'make all'\033[0m"
	@$(ECHO) "\033[0;33mRemember to do this before checkin, otherwise your CI will fail\033[0m"

.PHONY: banner_all
banner_all:
	@$(ECHO) "\033[1;37m====================\033[0m"
	@$(ECHO) "\033[1;37mPulumi Fabric (Full)\033[0m"
	@$(ECHO) "\033[1;37m====================\033[0m"

.PHONY: install
install:
	@$(ECHO) "\033[0;32mINSTALL:\033[0m"
	go install -ldflags "-X main.version=${VERSION}" ${PROJECT}
	go install -ldflags "-X main.version=${VERSION}" ${PROJECT}/cmd/lumidl

.PHONY: format
format:
	find . -iname "*.go" -not -path "./vendor/*" | xargs gofmt -s -w

.PHONY: lint
lint:
	@$(ECHO) "\033[0;32mLINT:\033[0m"
	$(GOMETALINTER) main.go | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./pkg/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./cmd/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/pulumi#191]: will fix when we write all of our API docs
#    - "Subprocess launching with variable": we intentionally launch processes dynamically.
#    - "cyclomatic complexity" (disabled in config): TODO[pulumi/pulumi#259]: need to fix many of these.
LINT_SUPPRESS="or be unexported|Subprocess launching with variable"

.PHONY: lint_quiet
lint_quiet:
	@$(ECHO) "\033[0;32mLINT (quiet):\033[0m"
	$(GOMETALINTER) main.go | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./pkg/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./cmd/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	@$(ECHO) "\033[0;33mlint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

.PHONY: vet
vet:
	@$(ECHO) "\033[0;32mVET:\033[0m"
	go tool vet -printf=false cmd/ pkg/

.PHONY: test
test:
	@$(ECHO) "\033[0;32mTEST:\033[0m"
	go test -timeout 2m -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

.PHONY: integrationtest
integrationtest:
	@$(ECHO) "\033[0;32mINTEGRATION TEST:\033[0m"
	@if [ -z "`which pulumi-langhost-nodejs`" ]; then $(ECHO) Please add "`pwd`/sdk/nodejs/bin" to your path before running integration tests. && exit 1; fi
	go test -cover -parallel ${TESTPARALLELISM} ./examples

sdk/nodejs:
	@cd ./sdk/nodejs && $(MAKE)
.PHONY: sdk/nodejs

publish:
	@$(ECHO) "\033[0;32mPublishing current release:\033[0m"
	./scripts/publish.sh
.PHONY: publish

.PHONY: gocover
gocover:
	@$(ECHO) "\033[0;32mGO CODE COVERAGE:\033[0m"
	./scripts/gocover.sh

# The travis_* targets are entrypoints for CI.
.PHONY: travis_cron
travis_cron: nightly

.PHONY: travis_push
travis_push: all publish

.PHONY: travis_pull_request
travis_pull_request: all

.PHONY: travis_api
travis_api: all
