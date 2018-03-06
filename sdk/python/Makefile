PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python
VERSION          := $(shell git describe --tags --dirty 2>/dev/null | sed "s/v//" | sed "s/-/+/" | sed "s/-/./g")

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

include ../../build/common.mk

ensure::
	$(PIP) install --user --upgrade -r requirements.txt

# Ensure Pylint is present and has the right version.
PYLINT := $(shell command -v pylint 2>/dev/null)
ifeq ($(PYLINT),)
ensure::
	$(error "missing pylint from your $$PATH; \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
else
PYLINT_VERSION := $(shell command $(PYLINT) --version 2>&1)
ifeq (,$(findstring Python 2.7,$(PYLINT_VERSION)))
ensure::
	$(error "$(PYLINT) did not report a 2.7 version number ($(PYLINT_VERSION)); \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
endif
endif

lint::
	$(PYLINT) -E lib/pulumi/ --ignore-patterns '.*_pb2_.*.py'
	$(GOMETALINTER) ./cmd/pulumi-language-python/... | sort ; exit $${PIPESTATUS[0]}
	$(GOMETALINTER) ./pkg/... | sort ; exit $${PIPESTATUS[0]}

build::
	cd ./lib/ && $(PYTHON) setup.py clean --all 2>/dev/null
	rm -rf ./bin/ && cp -R ./lib/. ./bin/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" ./bin/setup.py && rm ./bin/setup.py.bak
	cd ./bin/ && $(PYTHON) setup.py build bdist_wheel --universal
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install::
	cd ./bin/ && $(PYTHON) setup.py install --user --force --prefix=
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

test_fast::
