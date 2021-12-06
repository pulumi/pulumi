PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v3/python/cmd/pulumi-language-python
VERSION          := $(shell cd ../../ && pulumictl get version)
PYPI_VERSION 	 := $(shell cd ../../ && pulumictl get version --language python)
PROJECT_ROOT     := $(realpath ../..)

PYENV := ./env
PYENVSRC := $(PYENV)/src

PROJECT_PKGS    := $(shell go list ./cmd/...)
TESTPARALLELISM := 10

include ../../build/common.mk

# Motivation: running `make TEST_ALL_DEPS= test_all` permits running
# `test_all` without the dependencies.
TEST_ALL_DEPS = build

ensure::
	pipenv install --dev
	mkdir -p $(PYENVSRC)

build_package::
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak "s/\$${VERSION}/$(PYPI_VERSION)/g" $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	cp ../../README.md $(PYENVSRC)
	cd $(PYENVSRC) && pipenv run python setup.py build bdist_wheel --universal

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

lint::
	MYPYPATH=./stubs pipenv run mypy ./lib/pulumi --config-file=mypy.ini
	pipenv run pylint ./lib/pulumi --rcfile=.pylintrc

install_package:: build_package
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	cp ./dist/pulumi-resource-pulumi-python "$(PULUMI_BIN)"
	cp ./dist/pulumi-analyzer-policy-python "$(PULUMI_BIN)"

install_plugin:: build_plugin
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_package install_plugin

test_fast:: $(TEST_ALL_DEPS)
	$(GO_TEST) ${PROJECT_PKGS}
	pipenv install -e ./env/src
	# TODO the ignored test seems to fail in pytest but not unittest. Need to trackdown why
	$(TESTSUITE_SKIPPED) python/lib/test || \
		pipenv run coverage run -m pytest lib/test --ignore lib/test/automation --ignore lib/test/langhost/resource_thens/test_resource_thens.py
	$(TESTSUITE_SKIPPED) python/lib/test/langhost/resource_thens || \
		pipenv run coverage run -m unittest lib/test/langhost/resource_thens/test_resource_thens.py
	# Using python -m also adds lib/test_with_mocks to sys.path which avoids package resolution issues.
	pushd lib/test_with_mocks; $(TESTSUITE_SKIPPED) python/lib/test_with_mocks || pipenv run coverage run -m pytest; popd
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .coverage ]; then pipenv run coverage xml -o $(PULUMI_TEST_COVERAGE_PATH)/python-fast.xml; fi
endif

test_auto:: test_fast $(TEST_ALL_DEPS)
	# Note that this target depends on test-fast for the call to `pipenv run pip install`
	$(TESTSUITE_SKIPPED) auto-python || pipenv run coverage run -m pytest lib/test/automation
ifneq ($(PULUMI_TEST_COVERAGE_PATH),)
	if [ -e .coverage ]; then pipenv run coverage xml -o $(PULUMI_TEST_COVERAGE_PATH)/python-auto.xml; fi
endif

test_all:: test_fast test_auto

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
