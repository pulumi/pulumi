# syntax = docker/dockerfile:experimental
# Interim container so we can copy pulumi binaries
# Must be defined first
ARG PULUMI_VERSION=latest
ARG PULUMI_IMAGE=pulumi/pulumi-sdk-base
FROM ${PULUMI_IMAGE}:${PULUMI_VERSION} as pulumi

# The runtime container
FROM registry.access.redhat.com/ubi8/python-38:latest
WORKDIR /pulumi/projects

# Install needed tools, like git
RUN microdnf install -y \
    git \
    ca-certificates

# Uses the workdir, copies from pulumi interim container
COPY --from=pulumi /pulumi/bin/pulumi /pulumi/bin/pulumi
COPY --from=pulumi /pulumi/bin/*-python* /pulumi/bin/
ENV PATH "/pulumi/bin:${PATH}"

CMD ["pulumi"]
