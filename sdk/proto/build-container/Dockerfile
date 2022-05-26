# syntax = docker/dockerfile:experimental
FROM ubuntu:bionic

# This image was copied from https://github.com/pulumi/pulumi-build-container
# It can probably be simplifed as it is now purely used to generate the protobuf files

LABEL description="Pulumi Build Container"
LABEL maintainer="james@jen20.com"

RUN rm -f /etc/apt/apt.conf.d/docker-clean

COPY /scripts/utils.sh /build-scripts/

# Install various packages
COPY /scripts/install-packages.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    /build-scripts/install-packages.sh

# Install Go compiler
COPY /scripts/install-go.sh /build-scripts/
RUN GOLANG_VERSION=1.17 \
    GOLANG_SHA256=6bf89fc4f5ad763871cf7eac80a2d594492de7a818303283f1366a7f6a30372d \
    /build-scripts/install-go.sh

# Install Node.js and Yarn
COPY /scripts/install-node.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    NODE_VERSION=10.x \
    NODE_DISTRO=bionic \
    YARN_VERSION=1.21.1 \
    /build-scripts/install-node.sh

# Install Python and accoutrements
COPY /scripts/install-python.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    PYTHON_VERSION=3.7 \
    PIPENV_VERSION=2018.11.26 \
    AWSCLI_VERSION=1.17.9 \
    WHEEL_VERSION=0.34.2 \
    TWINE_VERSION=3.1.1 \
    /build-scripts/install-python.sh

# Install Protocol Buffers compiler and various GRPC generators
COPY /scripts/install-protobuf-tools.sh /build-scripts/
RUN PROTOC_VERSION=21.0 \
    PROTOC_SHA256=a2a92003da7b8c0c08aab530a3c1967d377c2777723482adb9d2eb38c87a9d5f \
    PROTOC_GEN_GO_VERSION=1.5.2 \
    NODEJS_GRPC_VERSION=1.24.2 \
    NODEJS_GRPC_TOOLS_VERSION=1.8.1 \
    PYTHON_GRPCIO_VERSION=1.27.2 \
    PYTHON_GRPCIO_TOOLS_VERSION=1.27.2 \
    /build-scripts/install-protobuf-tools.sh

# Set various required environment variables
ENV GOPATH=/go \
    PATH=/go/bin:/root/.pulumi/bin:/usr/local/go/bin:/root/.yarn/bin/:$PATH