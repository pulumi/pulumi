PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v3/python/cmd/pulumi-language-python
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh))
PYPI_VERSION 	   := $(if ${PYPI_VERSION},${PYPI_VERSION},$(shell ../../scripts/pulumi-version.sh python))

ifeq ($(DEBUG),"true")
$(info    VERSION       = $(VERSION))
$(info    PYPI_VERSION  = $(PYPI_VERSION))
endif

PYENV := ./env
PYENVSRC := $(PYENV)/src

BLACK_FLAGS := ./lib/pulumi --exclude lib/pulumi/runtime/proto

PROJECT_PKGS    := $(shell go list ./cmd/...)

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

GO_TEST      = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FLAGS)
GO_TEST_FAST = $(PYTHON) ../../scripts/go-test.py $(GO_TEST_FAST_FLAGS)

ensure:: $(PYTHON).ensure .ensure.phony
.ensure.phony:
	$(PYTHON) -m venv venv
	. venv/*/activate && python -m pip install --upgrade pip setuptools wheel
	. venv/*/activate && python -m pip install -r requirements.txt
	mkdir -p $(PYENVSRC)
	@touch .ensure.phony

build_package:: ensure
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak 's/^VERSION = .*/VERSION = "$(PYPI_VERSION)"/g' $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	cp ../../README.md $(PYENVSRC)
	. venv/*/activate && cd $(PYENVSRC) && \
		python setup.py build bdist_wheel --universal
	. venv/*/activate && python -m pip install -e $(PYENVSRC)

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

lint:: ensure
	. venv/*/activate && \
		python -m black $(BLACK_FLAGS) --check
	. venv/*/activate && \
		MYPYPATH=./stubs python -m mypy ./lib/pulumi --config-file=mypy.ini
	. venv/*/activate && \
		python -m pylint ./lib/pulumi --rcfile=.pylintrc

format:: ensure
	. venv/*/activate && \
		python -m black $(BLACK_FLAGS)

install_package:: build_package
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	cp ./dist/pulumi-resource-pulumi-python "$(PULUMI_BIN)"
	cp ./dist/pulumi-analyzer-policy-python "$(PULUMI_BIN)"

install_plugin:: build_plugin
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_package install_plugin

test_go:: $(TEST_ALL_DEPS)
	@$(GO_TEST) ${PROJECT_PKGS}

test_fast:: $(TEST_ALL_DEPS)
	. venv/*/activate && ./scripts/test_fast.sh

test_auto:: $(TEST_ALL_DEPS)
	. venv/*/activate && ./scripts/test_auto.sh

test_all:: test_fast test_auto test_go

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp ./cmd/pulumi-language-python-exec "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-resource-pulumi-python "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-analyzer-policy-python "$$(go env GOPATH)"/bin/

brew:: BREW_VERSION := $(shell ../../scripts/get-version HEAD)
brew::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${BREW_VERSION}" ${LANGHOST_PKG}
	cp ./cmd/pulumi-language-python-exec "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-resource-pulumi-python "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-analyzer-policy-python "$$(go env GOPATH)"/bin/

publish::
	for file in ../../artifacts/sdk-python-*.whl; do \
		basename=$$(basename $${file}); \
		mv -vT "$${file}" "../../artifacts/$${basename##sdk-python-}"; \
	done
	twine upload \
	-u pulumi -p "${PYPI_PASSWORD}" \
		../../artifacts/*.whl \
		--skip-existing \
		--verbose
