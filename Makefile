.SHELLFLAGS=-e

PROJECT=github.com/pulumi/lumi
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
TESTPARALLELISM=10

.PHONY: default
default: banner lint_quiet vet test install

.PHONY: all
all: banner_all lint_quiet vet test install lumijs lumirtpkg lumijspkg lumipkg awspkg

.PHONY: nightly
nightly: banner_all lint_quiet vet test install lumijs lumirtpkg lumijspkg lumipkg awspkg examples

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
	which gometalinter >/dev/null
	gometalinter pkg/... | sort ; exit "$${PIPESTATUS[0]}"
	gometalinter cmd/lumi/... | sort ; exit "$${PIPESTATUS[0]}"
	gometalinter cmd/lumidl/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/lumi#191]: will fix when we write all of our API docs
#    - "cyclomatic complexity": TODO[pulumi/lumi#259]: need to fix a bunch of cyclomatically complex functions.
#    - "Subprocess launching with variable": we intentionally launch processes dynamically.
LINT_SUPPRESS="or be unexported|cyclomatic complexity|Subprocess launching with variable"

.PHONY: lint_quiet
lint_quiet:
	@echo "\033[0;32mLINT (quiet):\033[0m"
	which gometalinter >/dev/null
	gometalinter pkg/... | grep -vE ${LINT_SUPPRESS} | sort
	gometalinter cmd/lumi/... | grep -vE ${LINT_SUPPRESS} | sort
	gometalinter cmd/lumidl/... | grep -vE ${LINT_SUPPRESS} | sort
	@test -z "$$(gometalinter pkg/... | grep -vE ${LINT_SUPPRESS})"
	@test -z "$$(gometalinter cmd/lumi/... | grep -vE ${LINT_SUPPRESS})"
	@test -z "$$(gometalinter cmd/lumidl/... | grep -vE ${LINT_SUPPRESS})"
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

