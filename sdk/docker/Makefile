LANGHOST_PKG     := github.com/pulumi/pulumi/sdk/docker/pulumi-language-docker/v3
VERSION          := $(if ${PULUMI_VERSION},${PULUMI_VERSION},$(shell ../../scripts/pulumi-version.sh))

include ../../build/common.mk

build:: ../../bin/pulumi-language-docker

.PHONY: ../../bin/pulumi-language-docker
../../bin/pulumi-language-docker:
	go build -C pulumi-language-docker -o ../../../bin/pulumi-language-docker \
		-ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
		${LANGHOST_PKG}

.PHONY: install
install:: ../../bin/pulumi-language-docker
	cp $< $(PULUMI_BIN)/pulumi-language-docker
