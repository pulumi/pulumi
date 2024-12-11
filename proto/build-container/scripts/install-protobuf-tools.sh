#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace

SCRIPT_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
#shellcheck source=utils.sh
source "${SCRIPT_ROOT}/utils.sh"

ensureSet "${PROTOC_VERSION}" "PROTOC_VERSION" || exit 1
ensureSet "${PROTOC_SHA256}" "PROTOC_SHA256" || exit 1
ensureSet "${PROTOC_GEN_GO_VERSION}" "PROTOC_GEN_GO_VERSION" || exit 1
ensureSet "${PROTOC_GEN_GO_GRPC_VERSION}" "PROTOC_GEN_GO_GRPC_VERSION" || exit 1
ensureSet "${NODEJS_GRPC_VERSION}" "NODEJS_GRPC_VERSION" || exit 1
ensureSet "${NODEJS_GRPC_TOOLS_VERSION}" "NODEJS_GRPC_TOOLS_VERSION" || exit 1
ensureSet "${NODEJS_TS_PROTOC_GEN_VERSION}" "NODEJS_TS_PROTOC_GEN_VERSION" || exit 1
ensureSet "${NODEJS_JS_PROTOC_GEN_VERSION}" "NODEJS_TS_PROTOC_GEN_VERSION" || exit 1
ensureSet "${PYTHON_GRPCIO_VERSION}" "PYTHON_GRPCIO_VERSION" || exit 1
ensureSet "${PYTHON_GRPCIO_TOOLS_VERSION}" "PYTHON_GRPCIO_TOOLS_VERSION" || exit 1

# Install Protocol Buffers Compiler
curl --silent -qL \
    -o /tmp/protoc.zip \
    "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"

verifySHASUM "/tmp/protoc.zip" "${PROTOC_SHA256}" || exit 1
mkdir -p /tmp/protoc
unzip /tmp/protoc.zip -d /tmp/protoc
mv /tmp/protoc/bin/protoc /usr/bin/protoc
mv /tmp/protoc/include/* /usr/include
rm -rf /tmp/protoc
rm -rf /tmp/protoc.zip

# Install protoc-gen-go
GOBIN=/usr/local/bin go install "google.golang.org/protobuf/cmd/protoc-gen-go@v${PROTOC_GEN_GO_VERSION}"
GOBIN=/usr/local/bin go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${PROTOC_GEN_GO_GRPC_VERSION}"

# Install Node gRPC Tools
npm install --unsafe-perm -g "@grpc/grpc-js@${NODEJS_GRPC_VERSION}" "grpc-tools@${NODEJS_GRPC_TOOLS_VERSION}" "grpc_tools_node_protoc_ts@${NODEJS_TS_PROTOC_GEN_VERSION}" protoc-gen-js@${NODEJS_JS_PROTOC_GEN_VERSION} 

# Install the Python gRPC Tools
python3 -m pip install "grpcio==${PYTHON_GRPCIO_VERSION}"
python3 -m pip install "grpcio-tools==${PYTHON_GRPCIO_TOOLS_VERSION}"
