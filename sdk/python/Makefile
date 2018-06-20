PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python
VERSION          := $(shell ../../scripts/get-py-version)

ifeq ($(PYTHON),)
	PYTHON := python
endif

ifeq ($(PIP),)
	PIP := pip
endif

ifneq ($(PYTHON_3),)
	PIPENV_ARGS := --three
else
	PIPENV_ARGS := --two
endif

PYENV := ./env
PYENVBIN := $(PYENV)/bin
PYENVSRC := $(PYENV)/src
PYENVLIB := $(PYENV)/lib

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

include ../../build/common.mk

ensure::
	$(PIP) install pipenv --user
	pipenv $(PIPENV_ARGS) install --dev
	mkdir -p $(PYENVSRC)
	
build::
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	cd $(PYENVSRC) && pipenv run python setup.py build bdist_wheel --universal
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

lint::
	pipenv run pylint -E ./lib/pulumi --ignore-patterns '.*_pb2_.*.py'
	$(GOMETALINTER) ./cmd/pulumi-language-python/... | sort ; exit $${PIPESTATUS[0]}
	$(GOMETALINTER) ./pkg/... | sort ; exit $${PIPESTATUS[0]}

install::
	cd $(PYENVSRC) && $(PIP) install --user -e .
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

test_fast::
	pipenv run pip install ./env/src
	pipenv run python -m unittest discover -s lib/test
