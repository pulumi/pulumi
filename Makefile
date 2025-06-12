PROJECT_NAME := Pulumi SDK
SDKS         ?= nodejs python go
SUB_PROJECTS := $(SDKS:%=sdk/%)

include build/common.mk

PROJECT         := github.com/pulumi/pulumi/pkg/v3/cmd/pulumi

# Ensure bin directory exists before targets are evaluated
# to avoid issues like realpath failing on macOS if the directory doesn't exist.
_ := $(shell mkdir -p bin)

PKG_CODEGEN := github.com/pulumi/pulumi/pkg/v3/codegen
# nodejs and python codegen tests are much slower than go/dotnet:
PROJECT_PKGS    := $(shell cd ./pkg && go list ./... | grep -v -E '^${PKG_CODEGEN}/(dotnet|go|nodejs|python)')
INTEGRATION_PKG := github.com/pulumi/pulumi/tests/integration
PERFORMANCE_PKG := github.com/pulumi/pulumi/tests/performance
TESTS_PKGS      := $(shell cd ./tests && go list -tags all ./... | grep -v tests/templates | grep -v ^${INTEGRATION_PKG}$ | grep -v ^${PERFORMANCE_PKG}$)
VERSION         := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ./scripts/pulumi-version.sh))

# Relative paths to directories with go.mod files that should be linted.
LINT_GOLANG_PKGS := sdk pkg tests sdk/go/pulumi-language-go sdk/nodejs/cmd/pulumi-language-nodejs sdk/python/cmd/pulumi-language-python cmd/pulumi-test-language

# Additional arguments to pass to golangci-lint.
GOLANGCI_LINT_ARGS ?=

ifeq ($(DEBUG),"true")
$(info    SHELL           = ${SHELL})
$(info    VERSION         = ${VERSION})
endif

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build $(SUB_PROJECTS:%=%_install)

# The number of Rapid checks to perform when fuzzing lifecycle tests. See the documentation on Rapid at
# https://pkg.go.dev/pgregory.net/rapid#section-readme or the lifecycle test documentation under
# pkg/engine/lifecycletest for more information.
LIFECYCLE_TEST_FUZZ_CHECKS ?= 1000

ensure: .make/ensure/go .make/ensure/phony $(SUB_PROJECTS:%=%_ensure)
.make/ensure/phony: sdk/go.mod pkg/go.mod tests/go.mod
	cd sdk && ../scripts/retry go mod download
	cd pkg && ../scripts/retry go mod download
	cd tests && ../scripts/retry go mod download
	@mkdir -p .make/ensure && touch .make/ensure/phony

.PHONY: build-proto build_proto
PROTO_FILES := $(sort $(shell find proto -type f -name '*.proto') proto/generate.sh proto/build-container/Dockerfile $(wildcard proto/build-container/scripts/*))
PROTO_CKSUM = cksum ${PROTO_FILES} | LC_ALL=C sort --key=3
build-proto: build_proto
build_proto:
	@printf "Protobuffer interfaces are ....... "
	@if [ "$$(cat proto/.checksum.txt)" = "`${PROTO_CKSUM}`" ]; then \
		printf "\033[0;32mup to date\033[0m\n"; \
	else \
		printf "\033[0;34mout of date: REBUILDING\033[0m\n"; \
		cd proto && ./generate.sh || exit 1; \
		cd ../ && ${PROTO_CKSUM} > proto/.checksum.txt; \
		printf "\033[0;34mProtobuffer interfaces have been \033[0;32mREBUILT\033[0m\n"; \
	fi

.PHONY: check-proto check_proto
check-proto: check_proto
check_proto:
	@if [ "$$(cat proto/.checksum.txt)" != "`${PROTO_CKSUM}`" ]; then \
		echo "Protobuf checksum doesn't match. Run \`make build_proto\` to rebuild."; \
		${PROTO_CKSUM} | diff - proto/.checksum.txt; \
		exit 1; \
	fi

.PHONY: generate
generate::
	$(call STEP_MESSAGE)
	echo "This command does not do anything anymore. It will be removed in a future version."

.PHONY: bin/pulumi
bin/pulumi: build_proto .make/ensure/go .make/ensure/phony
	go build -C pkg -o ../$@ -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${PROJECT}

build_display_wasm:: .make/ensure/go
	cd pkg && GOOS=js GOARCH=wasm go build -o ../bin/pulumi-display.wasm -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ./backend/display/wasm

.PHONY: build
build:: export GOBIN=$(shell realpath ./bin)
build:: build_proto .make/ensure/go dist build_display_wasm

install:: bin/pulumi
	cp $< $(PULUMI_BIN)/pulumi

build_debug::
	cd pkg && go install -gcflags="all=-N -l" -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${PROJECT}

build_cover::
	cd pkg && go build -cover -o ../bin/pulumi \
		-coverpkg github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/... \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${PROJECT}

install_cover:: build_cover
	cp bin/pulumi $(PULUMI_BIN)

developer_docs::
	cd developer-docs && make html

install_all:: install

dist::
	cd pkg && go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${PROJECT}

.PHONY: brew
# NOTE: the brew target intentionally avoids the dependency on `build`, as each language SDK has its own brew target
brew::
	./scripts/brew.sh "${PROJECT}"

.PHONY: lint_%
lint:: .make/ensure/golangci-lint lint_golang lint_pulumi_json

lint_pulumi_json::
	# NOTE: github.com/santhosh-tekuri/jsonschema uses Go's regexp engine, but
	# JSON schema says regexps should conform to ECMA 262.
	go run github.com/santhosh-tekuri/jsonschema/cmd/jv@v0.7.0 pkg/codegen/schema/pulumi.json
	cd sdk/nodejs && yarn biome format ../../pkg/codegen/schema/pulumi.json

lint_pulumi_json_fix::
	cd sdk/nodejs && yarn biome format --write ../../pkg/codegen/schema/pulumi.json

lint_fix:: lint_golang_fix lint_pulumi_json_fix

lint_golang:: lint_deps
	$(eval GOLANGCI_LINT_CONFIG = $(shell pwd)/.golangci.yml)
	@$(foreach pkg,$(LINT_GOLANG_PKGS),(cd $(pkg) && \
		echo "[golangci-lint] Linting $(pkg)..." && \
		golangci-lint run $(GOLANGCI_LINT_ARGS) \
			--config $(GOLANGCI_LINT_CONFIG) \
			--timeout 5m) \
		&&) true

lint_golang_fix::
	$(eval GOLANGCI_LINT_CONFIG = $(shell pwd)/.golangci.yml)
	@$(foreach pkg,$(LINT_GOLANG_PKGS),(cd $(pkg) && \
		echo "[golangci-lint] Linting $(pkg)..." && \
		golangci-lint run $(GOLANGCI_LINT_ARGS) \
			--config $(GOLANGCI_LINT_CONFIG) \
			--timeout 5m \
			--fix) \
		&&) true

lint_deps:
	@echo "Check for golangci-lint"; [ -e "$(shell which golangci-lint)" ]
lint_actions:
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.6.27 \
	  -format '{{range $$err := .}}### Error at line {{$$err.Line}}, col {{$$err.Column}} of `{{$$err.Filepath}}`\n\n{{$$err.Message}}\n\n```\n{{$$err.Snippet}}\n```\n\n{{end}}'

format:: ensure
	cd sdk/nodejs && yarn biome format --write ../../pkg/codegen/schema/pulumi.json

test_fast:: build get_schemas
	@cd pkg && $(GO_TEST_FAST) ${PROJECT_PKGS} ${PKG_CODEGEN_NODE}

test_all:: test_pkg test_integration

test_lifecycle:
	@cd pkg && $(GO_TEST) github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest

test_lifecycle_fuzz: GO_TEST_RACE = false
test_lifecycle_fuzz: export PULUMI_LIFECYCLE_TEST_FUZZ := 1
test_lifecycle_fuzz:
	@cd pkg && $(GO_TEST) github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest \
		-run '^TestFuzz$$' \
		-rapid.checks=$(LIFECYCLE_TEST_FUZZ_CHECKS)

test_lifecycle_fuzz_from_state_file: GO_TEST_RACE = false
test_lifecycle_fuzz_from_state_file:
	@cd pkg && $(GO_TEST) github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest \
		-run '^TestFuzzFromStateFile$$' \
		-rapid.checks=$(LIFECYCLE_TEST_FUZZ_CHECKS)

lang=$(subst test_codegen_,,$(word 1,$(subst !, ,$@)))
test_codegen_%: get_schemas
	@cd pkg && $(GO_TEST) ${PKG_CODEGEN}/${lang}/...

test_pkg_rest: get_schemas
	@cd pkg && $(GO_TEST) ${PROJECT_PKGS}

test_pkg:: test_pkg_rest test_codegen_dotnet test_codegen_go test_codegen_nodejs test_codegen_python

subset=$(subst test_integration_,,$(word 1,$(subst !, ,$@)))
test_integration_%:
	@cd tests && PULUMI_INTEGRATION_TESTS=$(subset) $(GO_TEST) $(INTEGRATION_PKG)

test_integration_subpkgs:
	@cd tests && $(GO_TEST) $(TESTS_PKGS)

test_integration:: $(SDKS:%=test_integration_%) test_integration_rest test_integration_subpkgs

test_performance:
	@cd tests && go test -count=1 -tags=all -timeout 1h -parallel=1 -v $(PERFORMANCE_PKG)

# Used by CI to run tests in parallel across the Go modules pkg, sdk, and tests.
.PHONY: gotestsum/%
gotestsum/%:
	cd $* && $(PYTHON) '$(CURDIR)/scripts/go-test.py' $(GO_TEST_FLAGS) $${OPTS} $${PKGS}

tidy::
	./scripts/tidy.sh --check

tidy_fix::
	./scripts/tidy.sh

validate_codecov_yaml::
	curl --data-binary @codecov.yml https://codecov.io/validate

# We replace the '!' with a space, then take the first word
# schema-pkg!x.y.z => schema-pkg
# We then replace 'schema-' with nothing, giving only the package name.
# schema-pkg => pkg
# Recall that `$@` is the target make is trying to build, in our case schema-pkg!x.y.z
name=$(subst schema-,,$(word 1,$(subst !, ,$@)))
# Here we take the second word, just the version
version=$(word 2,$(subst !, ,$@))
schema-%: .make/ensure/curl .make/ensure/jq
	@echo "Ensuring schema ${name}, ${version}"
	@# Download the package from github, then stamp in the correct version.
	@[ -f pkg/codegen/testing/test/testdata/${name}-${version}.json ] || \
		curl "https://raw.githubusercontent.com/pulumi/pulumi-${name}/v${version}/provider/cmd/pulumi-resource-${name}/schema.json" \
		| jq '.version = "${version}"' >  pkg/codegen/testing/test/testdata/${name}-${version}.json
	@# Confirm that the correct version is present. If not, error out.
	@FOUND="$$(jq -r '.version' pkg/codegen/testing/test/testdata/${name}-${version}.json)" &&        \
		if ! [ "$$FOUND" = "${version}" ]; then									           \
			echo "${name} required version ${version} but found existing version $$FOUND"; \
			exit 1;																		   \
		fi
# Related files:
#
# pkg/codegen/testing/utils/host.go depends on this list, update that file on changes.
#
# pkg/codegen/testing/test/helpers.go depends on some of this list, update that file on changes.
#
# pkg/codegen/schema/schema_test.go depends on kubernetes@3.7.2, update that file on changes.
#
# As a courtesy to reviewers, please make changes to this list and the committed schema files in a
# separate commit from other changes, as online code review tools may balk at rendering these diffs.
get_schemas: \
			schema-aws!4.15.0           \
			schema-aws!4.26.0           \
			schema-aws!4.36.0           \
			schema-aws!4.37.1           \
			schema-aws!5.4.0            \
			schema-aws!5.16.2           \
			schema-azure-native!1.28.0  \
			schema-azure-native!1.29.0  \
			schema-azure-native!1.56.0  \
			schema-azure!4.18.0         \
			schema-kubernetes!3.0.0     \
			schema-kubernetes!3.7.0     \
			schema-kubernetes!3.7.2     \
			schema-random!4.2.0         \
			schema-random!4.3.1         \
			schema-random!4.11.2        \
			schema-eks!0.37.1           \
			schema-eks!0.40.0           \
			schema-docker!3.1.0         \
			schema-docker!4.0.0-alpha.0 \
			schema-awsx!1.0.0-beta.5    \
			schema-aws-native!0.13.0    \
			schema-google-native!0.18.2 \
			schema-google-native!0.27.0 \
			schema-tls!4.10.0

.PHONY: changelog
changelog:
	go run github.com/pulumi/go-change@v0.1.3 create

.PHONY: work
work:
	rm -f go.work go.work.sum
	go work init \
		cmd/pulumi-test-language \
		pkg \
		sdk \
		sdk/go/pulumi-language-go \
		sdk/nodejs/cmd/pulumi-language-nodejs \
		sdk/python/cmd/pulumi-language-python \
		tests
