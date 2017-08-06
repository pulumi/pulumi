SHELL=/bin/bash
.SHELLFLAGS=-e

PROJECT=github.com/pulumi/pulumi-fabric
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
TESTPARALLELISM=10

ECHO=echo -e
GOMETALINTERBIN=gometalinter
GOMETALINTER=${GOMETALINTERBIN} --config=Gometalinter.json

.PHONY: default
default: banner vet install test lint_quiet

.PHONY: all
all: banner_all vet install test lint_quiet lumijs lumirtpkg lumijspkg lumipkg

.PHONY: nightly
nightly: all gocover

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
	go install ${PROJECT}/cmd/lumi
	go install ${PROJECT}/cmd/lumidl

.PHONY: lint
lint:
	@$(ECHO) "\033[0;32mLINT:\033[0m"
	$(GOMETALINTER) ./pkg/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./cmd/lumi/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./cmd/lumidl/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/pulumi-fabric#191]: will fix when we write all of our API docs
#    - "Subprocess launching with variable": we intentionally launch processes dynamically.
#    - "cyclomatic complexity" (disabled in config): TODO[pulumi/pulumi-fabric#259]: need to fix many of these.
LINT_SUPPRESS="or be unexported|Subprocess launching with variable"

.PHONY: lint_quiet
lint_quiet:
	@$(ECHO) "\033[0;32mLINT (quiet):\033[0m"
	$(GOMETALINTER) ./pkg/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./cmd/lumi/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	$(GOMETALINTER) ./cmd/lumidl/... | grep -vE ${LINT_SUPPRESS} | sort ; exit $$(($${PIPESTATUS[1]}-1))
	@$(ECHO) "\033[0;33mlint was run quietly; to run with noisy errors, run 'make lint'\033[0m"

.PHONY: vet
vet:
	@$(ECHO) "\033[0;32mVET:\033[0m"
	go tool vet -printf=false cmd/ pkg/

.PHONY: test
test:
	@$(ECHO) "\033[0;32mTEST:\033[0m"
	go test -cover -parallel ${TESTPARALLELISM} ${PROJECT_PKGS} ./examples

.PHONY: lumijs
lumijs:
	@cd ./cmd/lumijs && $(MAKE)

lumirtpkg:
	@cd ./lib/lumirt && $(MAKE)
lumijspkg:
	@cd ./lib/lumijs && $(MAKE)
lumipkg:
	@cd ./lib/lumi && $(MAKE)
.PHONY: lumirtpkg lumijspkg lumipkg

publish:
	@$(ECHO) "\033[0;32mPublishing current release:\033[0m"
	./scripts/publish.sh
.PHONY: publish

.PHONY: gocover
gocover:
	@$(ECHO) "\033[0;32mGO CODE COVERAGE:\033[0m"
	./scripts/gocover.sh

