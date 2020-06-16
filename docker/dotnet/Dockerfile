# syntax = docker/dockerfile:experimental
# Interim container so we can copy pulumi binaries
# Must be defined first
ARG PULUMI_VERSION=latest
ARG PULUMI_IMAGE=pulumi/pulumi-sdk-base
FROM ${PULUMI_IMAGE}:${PULUMI_VERSION} as pulumi

# The runtime container
FROM registry.access.redhat.com/ubi8/dotnet-31-runtime:latest
WORKDIR /pulumi/projects

RUN microdnf install -y \
    ca-certificates \
    git

# Uses the workdir, copies from pulumi interim container
COPY --from=pulumi /pulumi/bin/pulumi /pulumi/bin/pulumi
COPY --from=pulumi /pulumi/bin/*-dotnet* /pulumi/bin/
ENV PATH "/pulumi/bin:${PATH}"

CMD ["pulumi"]
