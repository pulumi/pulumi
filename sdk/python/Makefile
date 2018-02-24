PROJECT_NAME     := Pulumi Python SDK
LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python
VERSION          := $(shell git describe --tags --dirty 2>/dev/null | sed "s/v//" | sed "s/-/+/" | sed "s/-/./g")

GOMETALINTERBIN := gometalinter
GOMETALINTER    := ${GOMETALINTERBIN} --config=../../Gometalinter.json

include ../../build/common.mk

ensure::
	pip install -r requirements.txt

lint::
	pylint -E lib/pulumi/ --ignore-patterns '.*_pb2_.*.py'
	$(GOMETALINTER) ./cmd/pulumi-language-python/... | sort ; exit $${PIPESTATUS[0]}
	$(GOMETALINTER) ./pkg/... | sort ; exit $${PIPESTATUS[0]}

build::
	cd ./lib/ && python setup.py clean --all 2>/dev/null
	rm -rf ./bin/ && cp -R ./lib/. ./bin/
	sed -i.bak "s/\$${VERSION}/$(VERSION)/g" ./bin/setup.py && rm ./bin/setup.py.bak
	cd ./bin/ && python setup.py build
	go install -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

install::
	cd ./bin/ && python setup.py install --force
	cp ./cmd/pulumi-language-python-exec "$(PULUMI_BIN)"
	GOBIN=$(PULUMI_BIN) go install \
		  -ldflags "-X github.com/pulumi/pulumi/sdk/python/pkg/version.Version=${VERSION}" ${LANGHOST_PKG}

test_fast::
