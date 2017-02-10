#!/bin/sh
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
#         go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
#
# The results are checked into bin/; at this moment, they need to be copied to their final destinations manually.
set -e

mkdir -p bin/
protoc --go_out=plugins=grpc:bin/ *.proto

