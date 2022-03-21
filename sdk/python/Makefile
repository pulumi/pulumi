PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v3/python/cmd/pulumi-language-python
VERSION          := $(shell cd ../../ && pulumictl get version)
PYPI_VERSION 	 := $(shell cd ../../ && pulumictl get version --language python)
PROJECT_ROOT     := $(realpath ../..)

PYENV := ./env
PYENVSRC := $(PYENV)/src

PROJECT_PKGS    := $(shell go list ./cmd/...)
TESTPARALLELISM ?= 10

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS ?= build

ensure::
	$(PYTHON) -m venv venv
	. venv/*/activate && python -m pip install wheel
	. venv/*/activate && python -m pip install -r requirements.txt
	mkdir -p $(PYENVSRC)

build_package:: ensure
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak "s/\$${VERSION}/$(PYPI_VERSION)/g" $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	cp ../../README.md $(PYENVSRC)
	. venv/*/activate && cd $(PYENVSRC) && \
		python setup.py build bdist_wheel --universal
	. venv/*/activate && python -m pip install -e $(PYENVSRC)

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

lint:: ensure
	. venv/*/activate && \
		python -m black ./lib/pulumi --check
	. venv/*/activate && \
		MYPYPATH=./stubs python -m mypy ./lib/pulumi --config-file=mypy.ini
	. venv/*/activate && \
		python -m pylint ./lib/pulumi --rcfile=.pylintrc

format:: ensure
	. venv/*/activate && \
		python -m black ./lib/pulumi

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

publish:: build_package
	twine upload \
	-u pulumi -p "${PYPI_PASSWORD}" \
        "env/src/dist"/*.whl \
        --skip-existing \
        --verbose
