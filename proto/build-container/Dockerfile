# syntax = docker/dockerfile:experimental
FROM ubuntu:focal

# This image was copied from https://github.com/pulumi/pulumi-build-container
# It can probably be simplified as it is now purely used to generate the protobuf files

LABEL description="Pulumi Protobuf Build Container"

RUN rm -f /etc/apt/apt.conf.d/docker-clean

COPY /scripts/utils.sh /build-scripts/

# Install various packages
COPY /scripts/install-packages.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    /build-scripts/install-packages.sh

# Install Go compiler
COPY /scripts/install-go.sh /build-scripts/
RUN GOLANG_VERSION=1.22.10 \
    GOLANG_SHA256=736ce492a19d756a92719a6121226087ccd91b652ed5caec40ad6dbfb2252092 \
    /build-scripts/install-go.sh

# Install Node.js and Yarn
COPY /scripts/install-node.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    NODE_VERSION=18.x \
    NODE_DISTRO=focal \
    YARN_VERSION=3.8.7 \
    /build-scripts/install-node.sh

# Install Python and accoutrements
COPY /scripts/install-python.sh /build-scripts/
RUN --mount=target=/var/lib/apt/lists,type=cache \
    --mount=target=/var/cache/apt,type=cache \
    PYTHON_VERSION=3.9 \
    PIPENV_VERSION=2024.4.0 \
    AWSCLI_VERSION=1.36.19 \
    WHEEL_VERSION=0.45.1 \
    TWINE_VERSION=3.8.0 \
    MYPY_VERSION=3.6.0 \
    /build-scripts/install-python.sh

# Set various required environment variables
ENV GOPATH=/go \
    PATH=/root/.local/bin:/go/bin:/root/.pulumi/bin:/usr/local/go/bin:/root/.yarn/bin/:$PATH

# Install Protocol Buffers compiler and various GRPC generators
COPY /scripts/install-protobuf-tools.sh /build-scripts/
RUN PROTOC_VERSION=31.0 \
    PROTOC_SHA256=24e2ed32060b7c990d5eb00d642fde04869d7f77c6d443f609353f097799dd42 \
    PROTOC_GEN_GO_VERSION=1.36.6 \
    PROTOC_GEN_GO_GRPC_VERSION=1.5.1 \
    NODEJS_GRPC_VERSION=1.13.3 \
    NODEJS_GRPC_TOOLS_VERSION=1.13.0 \
    NODEJS_TS_PROTOC_GEN_VERSION=5.3.3 \
    NODEJS_JS_PROTOC_GEN_VERSION=3.21.4 \
    PYTHON_GRPCIO_VERSION=1.71.0 \
    PYTHON_GRPCIO_TOOLS_VERSION=1.71.0 \
    /build-scripts/install-protobuf-tools.sh
