# syntax = docker/dockerfile:experimental
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest as builder
ARG PULUMI_VERSION=latest
RUN microdnf install -y \
      curl \
      make \
      gcc \
      git \
      gcc-c++
# Install the Pulumi SDK, including the CLI and language runtimes.
RUN if [ "$PULUMI_VERSION" = "latest" ]; then \
    curl -fsSL https://get.pulumi.com/ | bash; \
  else \
    curl -fsSL https://get.pulumi.com/ | bash -s -- --version $PULUMI_VERSION ; \
  fi

# The runtime container
# This is our base container, so let's copy all the runtimes to .pulumi/bin
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
WORKDIR /pulumi
COPY --from=builder /root/.pulumi/bin bin

RUN microdnf install -y \
    git \
    ca-certificates

ENV PATH "/pulumi/bin:${PATH}"
CMD ["pulumi"]
