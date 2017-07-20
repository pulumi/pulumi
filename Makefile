SHELL=/bin/bash
.SHELLFLAGS=-e

PROJECT=github.com/pulumi/lumi
PROJECT_PKGS=$(shell go list ./cmd/... ./pkg/... | grep -v /vendor/)
LUMIROOT ?= /usr/local/lumi
LUMILIB   = ${LUMIROOT}/packs
TESTPARALLELISM=10

ECHO=echo -e
GOMETALINTERBIN=gometalinter
GOMETALINTER=${GOMETALINTERBIN} --config=Gometalinter.json

.PHONY: default
default: banner vet test install lint_quiet

.PHONY: all
all: banner_all vet test install lint_quiet lumijs lumirtpkg lumijspkg lumipkg awspkg

.PHONY: nightly
nightly: banner_all vet test install lint_quiet lumijs lumirtpkg lumijspkg lumipkg awspkg examples gocover

.PHONY: banner
banner:
	@$(ECHO) "\033[1;37m============\033[0m"
	@$(ECHO) "\033[1;37mLumi (Quick)\033[0m"
	@$(ECHO) "\033[1;37m============\033[0m"
	@$(ECHO) "\033[0;33mRunning quick build; to run full tests, run 'make all'\033[0m"
	@$(ECHO) "\033[0;33mRemember to do this before checkin, otherwise your CI will fail\033[0m"

.PHONY: banner_all
banner_all:
	@$(ECHO) "\033[1;37m============\033[0m"
	@$(ECHO) "\033[1;37mLumi (Full)\033[0m"
	@$(ECHO) "\033[1;37m============\033[0m"

.PHONY: install
install:
	@$(ECHO) "\033[0;32mINSTALL:\033[0m"
	go install ${PROJECT}/cmd/lumi
	go install ${PROJECT}/cmd/lumidl

.PHONY: lint
lint:
	@$(ECHO) "\033[0;32mLINT:\033[0m"
	which ${GOMETALINTERBIN} >/dev/null
	$(GOMETALINTER) ./pkg/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./cmd/lumi/... | sort ; exit "$${PIPESTATUS[0]}"
	$(GOMETALINTER) ./cmd/lumidl/... | sort ; exit "$${PIPESTATUS[0]}"

# In quiet mode, suppress some messages.
#    - "or be unexported": TODO[pulumi/lumi#191]: will fix when we write all of our API docs
#    - "Subprocess launching with variable": we intentionally launch processes dynamically.
#    - "cyclomatic complexity" (disabled in config): TODO[pulumi/lumi#259]: need to fix many of these.
LINT_SUPPRESS="or be unexported|Subprocess launching with variable"

.PHONY: lint_quiet
lint_quiet:
	@$(ECHO) "\033[0;32mLINT (quiet):\033[0m"
	which ${GOMETALINTERBIN} >/dev/null
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

PUBDIR := $(shell mktemp -du)
GITVER := $(shell git rev-parse HEAD)
PUBFILE := $(shell dirname ${PUBDIR})/${GITVER}.tgz
PUBTARGET := "s3://eng.pulumi.com/releases/${GITVER}.tgz"
publish:
	@git diff-index --quiet HEAD -- || \
		test -n "${PUBFORCE}" || \
		(echo "error: Cannot publish a dirty repo; set PUBFORCE=true to override" && exit 99)
	@$(ECHO) Publishing to: ${PUBTARGET}
	mkdir -p ${PUBDIR}/cmd ${PUBDIR}/packs
	cp ${GOPATH}/bin/lumi ${PUBDIR}/cmd
	cp -R ${LUMILIB}/lumirt ${PUBDIR}/packs/lumirt
	cp -R ${LUMILIB}/lumijs ${PUBDIR}/packs/lumijs
	cp -R ${LUMILIB}/lumi ${PUBDIR}/packs/lumi
	tar -czf ${PUBFILE} -C ${PUBDIR} .
	aws s3 cp ${PUBFILE} ${PUBTARGET}
.PHONY: publish

.PHONY: examples
examples:
	@$(ECHO) "\033[0;32mTEST EXAMPLES:\033[0m"
	go test -v -cover -timeout 1h -parallel ${TESTPARALLELISM} ./examples

.PHONY: gocover
gocover:
	@$(ECHO) "\033[0;32mGO CODE COVERAGE:\033[0m"
	./gocover.sh

