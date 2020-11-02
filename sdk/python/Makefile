PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/v2/python/cmd/pulumi-language-python
VERSION          := $(shell ../../scripts/get-version HEAD)
PYPI_VERSION     := $(shell cd ../../scripts && ./get-py-version HEAD)

PYENV := ./env
PYENVSRC := $(PYENV)/src

include ../../build/common.mk

ensure::
	pipenv install --dev
	mkdir -p $(PYENVSRC)

build_package::
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak "s/\$${VERSION}/$(PYPI_VERSION)/g" $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	cp ../../README.md $(PYENVSRC)
	cd $(PYENVSRC) && pipenv run python setup.py build bdist_wheel --universal

build_plugin::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

build:: build_package build_plugin

lint::
	pipenv run mypy ./lib/pulumi --config-file=mypy.ini
	pipenv run pylint ./lib/pulumi --rcfile=.pylintrc

install_package:: build_package
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	cp ./dist/pulumi-resource-pulumi-python "$(PULUMI_BIN)"
	cp ./dist/pulumi-analyzer-policy-python "$(PULUMI_BIN)"

install_plugin:: build_plugin
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}

install:: install_package install_plugin

test_fast:: build
	pipenv run pip install ./env/src
	pipenv run python -m unittest discover -s lib/test -v
	pipenv run python -m unittest discover -s lib/test_with_mocks -v

test_all:: test_fast

dist::
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/v2/go/common/version.Version=${VERSION}" ${LANGHOST_PKG}
	cp ./cmd/pulumi-language-python-exec "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-resource-pulumi-python "$$(go env GOPATH)"/bin/
	cp ./dist/pulumi-analyzer-policy-python "$$(go env GOPATH)"/bin/

brew:: dist

publish:: build_package
	twine upload \
    	-u pulumi -p "${PYPI_PASSWORD}" \
        "env/src/dist"/*.whl \
        --skip-existing \
        --verbose
