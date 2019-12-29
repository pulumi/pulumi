// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2018, Pulumi Corporation.
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
var grpc = require('grpc');
var runtime_pb = require('./runtime_pb.js');
var plugin_pb = require('./plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_pulumirpc_ConstructRequest(arg) {
  if (!(arg instanceof runtime_pb.ConstructRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConstructRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConstructRequest(buffer_arg) {
  return runtime_pb.ConstructRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConstructResponse(arg) {
  if (!(arg instanceof runtime_pb.ConstructResponse)) {
    throw new Error('Expected argument of type pulumirpc.ConstructResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConstructResponse(buffer_arg) {
  return runtime_pb.ConstructResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var RuntimeService = exports.RuntimeService = {
  construct: {
    path: '/pulumirpc.Runtime/Construct',
    requestStream: false,
    responseStream: false,
    requestType: runtime_pb.ConstructRequest,
    responseType: runtime_pb.ConstructResponse,
    requestSerialize: serialize_pulumirpc_ConstructRequest,
    requestDeserialize: deserialize_pulumirpc_ConstructRequest,
    responseSerialize: serialize_pulumirpc_ConstructResponse,
    responseDeserialize: deserialize_pulumirpc_ConstructResponse,
  },
};

exports.RuntimeClient = grpc.makeGenericClientConstructor(RuntimeService);
