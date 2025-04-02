# syntax = docker/dockerfile:experimental
FROM ubuntu:bionic

# This image was copied from https://github.com/pulumi/pulumi-build-container
# It can probably be simplified as it is now purely used to generate the protobuf files

LABEL description="Pulumi Protobuf Build Container"

RUN rm -f /etc/apt/apt.conf.d/docker-clean

RUN echo "Acquire::http::Pipeline-Depth 0; \n Acquire::http::No-Cache true; \n Acquire::BrokenProxy    true;" > /etc/apt/apt.conf.d/99fixbadproxy


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
    MYPY_VERSION=3.4.0 \
    /build-scripts/install-python.sh

# Set various required environment variables
ENV GOPATH=/go \
    PATH=/root/.local/bin:/go/bin:/root/.pulumi/bin:/usr/local/go/bin:/root/.yarn/bin/:$PATH

# Install Protocol Buffers compiler and various GRPC generators
COPY /scripts/install-protobuf-tools.sh /build-scripts/
RUN PROTOC_VERSION=3.20.1 \
    PROTOC_SHA256=3a0e900f9556fbcac4c3a913a00d07680f0fdf6b990a341462d822247b265562 \
    PROTOC_GEN_GO_VERSION=1.28.1 \
    PROTOC_GEN_GO_GRPC_VERSION=1.2.0 \
    NODEJS_GRPC_VERSION=1.24.2 \
    NODEJS_GRPC_TOOLS_VERSION=1.12.4 \
    NODEJS_TS_PROTOC_GEN_VERSION=5.3.3 \
    PYTHON_GRPCIO_VERSION=1.50.0 \
    PYTHON_GRPCIO_TOOLS_VERSION=1.50.0 \
    /build-scripts/install-protobuf-tools.sh
