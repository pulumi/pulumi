PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python
VERSION          := $(shell ../../scripts/get-version | sed "s/v//" | sed "s/-dev-/a/" | sed "s/-rc/rc/")

PYENV := env
PYENVBIN := $(PYENV)/bin
PYENVSRC := $(PYENV)/src
PYENVLIB := $(PYENV)/lib

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

include ../../build/common.mk

ensure::
	$(PIP) install 'virtualenv>=15.2.0'
	rm -rf $(PYENV) && virtualenv --python=$(PYTHON) $(PYENV)/
	echo "export PYTHONPATH=$${PYTHONPATH}:$(shell pwd)/$(PYENVLIB)/python2.7/site-packages" >> $(PYENVBIN)/activate
	( \
		source $(PYENVBIN)/activate ; \
		pip install -r requirements.txt ; \
	)
	mkdir -p $(PYENVSRC)

build::
	rm -rf $(PYENVSRC) && cp -R ./lib/. $(PYENVSRC)/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" $(PYENVSRC)/setup.py && rm $(PYENVSRC)/setup.py.bak
	( \
		source $(PYENVBIN)/activate ; \
		cd $(PYENVSRC) && python setup.py build bdist_wheel --universal ; \
	)
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

lint::
	( \
		source $(PYENVBIN)/activate ; \
		cd $(PYENVSRC) && pylint -E ./pulumi --ignore-patterns '.*_pb2_.*.py' ; \
	)
	$(GOMETALINTER) ./cmd/pulumi-language-python/... | sort ; exit $${PIPESTATUS[0]}
	$(GOMETALINTER) ./pkg/... | sort ; exit $${PIPESTATUS[0]}

install::
	cd $(PYENVSRC) && $(PIP) install --user -e .
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

test_fast::
