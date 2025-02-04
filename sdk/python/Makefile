PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh))
PYPI_VERSION 	   := $(if ${PYPI_VERSION},${PYPI_VERSION},$(shell ../../scripts/pulumi-version.sh python))

ifeq ($(DEBUG),"true")
$(info    VERSION       = $(VERSION))
$(info    PYPI_VERSION  = $(PYPI_VERSION))
endif

PROJECT_PKGS    := $(shell go list -C cmd/pulumi-language-python ./...)

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

ensure:: .make/ensure/uv
	uv venv --allow-existing
	uv sync --dev

build_package:: ensure
	uv run -m build --outdir ./build --installer uv

build_plugin::
	go install -C cmd/pulumi-language-python \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

lint:: ensure
	uv run -m ruff format --check
	MYPYPATH=./stubs uv run -m mypy ./lib/pulumi
	uv run -m ruff check
	uv run -m pyright

lint_fix:: ensure
	make format
	make lint

format:: ensure
	uv run -m ruff format

install_package:: build_package
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	cp ./dist/pulumi-resource-pulumi-python "$(PULUMI_BIN)"
	cp ./dist/pulumi-analyzer-policy-python "$(PULUMI_BIN)"

install_plugin:: build_plugin
	GOBIN=$(PULUMI_BIN) go install -C cmd/pulumi-language-python \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_package install_plugin

test_go:: $(TEST_ALL_DEPS)
	@cd cmd/pulumi-language-python && \
		$(GO_TEST) ${PROJECT_PKGS}

test_fast:: $(TEST_ALL_DEPS)
	. .venv/*/activate && ./scripts/test_fast.sh

test_auto:: $(TEST_ALL_DEPS)
	. .venv/*/activate && ./scripts/test_auto.sh

test_all:: test_fast test_auto test_go

dist::
	export GOBIN=$(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)
	go install -C cmd/pulumi-language-python \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp ./cmd/pulumi-language-python-exec "$GOBIN"
	cp ./dist/pulumi-resource-pulumi-python "$GOBIN"
	cp ./dist/pulumi-analyzer-policy-python "$GOBIN"

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -C cmd/pulumi-language-python \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}
	cp ./cmd/pulumi-language-python-exec "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-resource-pulumi-python "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-analyzer-policy-python "$$(go env GOPATH)"/bin/

publish:: ensure
	for file in ../../artifacts/sdk-python-*.whl; do \
		basename=$$(basename $${file}); \
		mv -vT "$${file}" "../../artifacts/$${basename##sdk-python-}"; \
	done
	uv run twine upload \
	-u "${PYPI_USERNAME}" -p "${PYPI_PASSWORD}" \
		../../artifacts/*.whl \
		--skip-existing \
		--verbose
