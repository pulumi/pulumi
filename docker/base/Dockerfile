# syntax = docker/dockerfile:experimental
FROM debian:buster-slim AS builder
ARG PULUMI_VERSION=latest
RUN apt-get update -y && \
      apt-get upgrade -y && \
      apt-get install -y \
      curl \
      build-essential \
      git

# Install the Pulumi SDK, including the CLI and language runtimes.
RUN if [ "$PULUMI_VERSION" = "latest" ]; then \
    curl -fsSL https://get.pulumi.com/ | bash; \
  else \
    curl -fsSL https://get.pulumi.com/ | bash -s -- --version "${PULUMI_VERSION##v}" ; \
  fi

# The runtime container
# This is our base container, so let's copy all the runtimes to .pulumi/bin
FROM debian:buster-slim
WORKDIR /pulumi
COPY --from=builder /root/.pulumi/bin bin

RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y \
    git \
    ca-certificates

ENV PATH "/pulumi/bin:${PATH}"
CMD ["pulumi"]
