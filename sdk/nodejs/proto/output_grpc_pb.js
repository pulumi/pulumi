// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
'use strict';
var grpc = require('@grpc/grpc-js');
var pulumi_output_pb = require('./output_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetCapabilitiesResponse(arg) {
  if (!(arg instanceof pulumi_output_pb.GetCapabilitiesResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetCapabilitiesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetCapabilitiesResponse(buffer_arg) {
  return pulumi_output_pb.GetCapabilitiesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_WriteRequest(arg) {
  if (!(arg instanceof pulumi_output_pb.WriteRequest)) {
    throw new Error('Expected argument of type pulumirpc.WriteRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_WriteRequest(buffer_arg) {
  return pulumi_output_pb.WriteRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


// Output is used to display sub-process output back to a host process. We can't send file descriptors over
// grpc safely (especially if we ever have remote plugins), so this is a small service to allow us to send
// back standard output and error data. For clients that support it, this also exposes a property to say if
// they should behave as if the host is a terminal.
var OutputService = exports.OutputService = {
  // Returns the capabilities of the output, such as if it is a terminal.
getCapabilities: {
    path: '/pulumirpc.Output/GetCapabilities',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_output_pb.GetCapabilitiesResponse,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_GetCapabilitiesResponse,
    responseDeserialize: deserialize_pulumirpc_GetCapabilitiesResponse,
  },
  // Write to the output.
write: {
    path: '/pulumirpc.Output/Write',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_output_pb.WriteRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_WriteRequest,
    requestDeserialize: deserialize_pulumirpc_WriteRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.OutputClient = grpc.makeGenericClientConstructor(OutputService);
