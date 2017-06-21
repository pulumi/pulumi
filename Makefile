.SHELLFLAGS=-e

PROJECT=github.com/pulumi/lumi
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
TESTPARALLELISM=10

GOMETALINTERBIN=gometalinter
GOMETALINTER=${GOMETALINTERBIN} --disable=gotype

.PHONY: default
default: banner vet test install lint_quiet

.PHONY: all
all: banner_all vet test install lint_quiet lumijs lumirtpkg lumijspkg lumipkg awspkg

.PHONY: nightly
nightly: banner_all vet test install lint_quiet lumijs lumirtpkg lumijspkg lumipkg awspkg examples

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

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m"
	go install ${PROJECT}/cmd/lumi
	go install ${PROJECT}/cmd/lumidl

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	which ${GOMETALINTERBIN} >/dev/null
	$(GOMETALINTER) pkg/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) cmd/lumi/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) cmd/lumidl/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/lumi#191]: will fix when we write all of our API docs
#    - "cyclomatic complexity": TODO[pulumi/lumi#259]: need to fix a bunch of cyclomatically complex functions.
#    - "Subprocess launching with variable": we intentionally launch processes dynamically.
LINT_SUPPRESS="or be unexported|cyclomatic complexity|Subprocess launching with variable"

.PHONY: lint_quiet
lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	which ${GOMETALINTERBIN} >/dev/null
	$(GOMETALINTER) pkg/... | grep -vE ${LINT_SUPPRESS} | sort
	$(GOMETALINTER) cmd/lumi/... | grep -vE ${LINT_SUPPRESS} | sort
	$(GOMETALINTER) cmd/lumidl/... | grep -vE ${LINT_SUPPRESS} | sort
	@test -z "$$($(GOMETALINTER) pkg/... | grep -vE ${LINT_SUPPRESS})"
	@test -z "$$($(GOMETALINTER) cmd/lumi/... | grep -vE ${LINT_SUPPRESS})"
	@test -z "$$($(GOMETALINTER) cmd/lumidl/... | grep -vE ${LINT_SUPPRESS})"
	@echo "\033[0;33mlint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

.PHONY: vet
vet:
	@echo "\033[0;32mVET:\033[0m"
	go tool vet -printf=false cmd/ pkg/

.PHONY: test
test:
	@echo "\033[0;32mTEST:\033[0m"
	go test -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS}

.PHONY: lumijs
lumijs:
	@cd ./cmd/lumijs && $(MAKE)

.PHONY: lumirtpkg
lumirtpkg:
	@cd ./lib/lumirt && $(MAKE)

.PHONY: lumijspkg
lumijspkg:
	@cd ./lib/lumijs && $(MAKE)

.PHONY: lumipkg
lumipkg:
	@cd ./lib/lumi && $(MAKE)

.PHONY: awspkg
awspkg:
	@cd ./lib/aws && $(MAKE)

.PHONY: verify
verify:
	@cd ./lib/aws && $(MAKE) verify

.PHONY: examples
examples:
	@echo "\033[0;32mTEST EXAMPLES:\033[0m"
	go test -v -cover -timeout 1h -parallel ${TESTPARALLELISM} ./examples

