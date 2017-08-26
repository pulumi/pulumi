#!/bin/bash
# This script regenerates all Protobuf/gRPC client files.
#
# For now, it must be run manually, and the results are checked into source control.  Eventually we might choose to
# automate this process as part of the overall build so that it's less manual and hence error prone.
#
# To run this script, the following pre-requisites are necessary:
#
#     1) Install the latest Protobuf compiler from https://github.com/google/protobuf/releases.
#     2) Add the `protoc` binary to your PATH (so that it can be found below).
#     3) Install the Golang Protobuf compiler by running this command from your Go workspace (also on your PATH):
#            go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
#     4) Install the Node.js gRPC SDK, which includes the gRPC Node.js compiler plugin
#            npm install -g grpc-tools
#        and add the `grpc_tools_node_protoc_plugin` binary to your PATH.
#
# The results are checked into bin/; at this moment, they need to be copied to their final destinations manually.
set -e

PROTOC=$(which protoc || { >&2 echo "error: Protobuf compiler (protoc) not found on PATH"; exit 1; })

echo Generating Protobuf/gRPC SDK files:

GO_LUMIRPC=./go
GO_PROTOFLAGS="plugins=grpc"
echo -e "\tGo: $GO_LUMIRPC [$GO_PROTOFLAGS]"
mkdir -p $GO_LUMIRPC
$PROTOC --go_out=$GO_PROTOFLAGS:$GO_LUMIRPC *.proto

JS_LUMIRPC=./nodejs
JS_PROTOFLAGS="import_style=commonjs,binary"
echo -e "\tJS: $JS_LUMIRPC [$JS_PROTOFLAGS]"
mkdir -p $JS_LUMIRPC
$PROTOC --js_out=$JS_PROTOFLAGS:$JS_LUMIRPC --grpc_out=$JS_LUMIRPC --plugin=protoc-gen-grpc=`which grpc_tools_node_protoc_plugin` *.proto

echo Done.

