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
var grpc = require('@grpc/grpc-js');
var pulumi_engine_pb = require('./engine_pb.js');
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

function serialize_pulumirpc_GetRootResourceRequest(arg) {
  if (!(arg instanceof pulumi_engine_pb.GetRootResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetRootResourceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRootResourceRequest(buffer_arg) {
  return pulumi_engine_pb.GetRootResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRootResourceResponse(arg) {
  if (!(arg instanceof pulumi_engine_pb.GetRootResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetRootResourceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRootResourceResponse(buffer_arg) {
  return pulumi_engine_pb.GetRootResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_LogRequest(arg) {
  if (!(arg instanceof pulumi_engine_pb.LogRequest)) {
    throw new Error('Expected argument of type pulumirpc.LogRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_LogRequest(buffer_arg) {
  return pulumi_engine_pb.LogRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_SetRootResourceRequest(arg) {
  if (!(arg instanceof pulumi_engine_pb.SetRootResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.SetRootResourceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_SetRootResourceRequest(buffer_arg) {
  return pulumi_engine_pb.SetRootResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_SetRootResourceResponse(arg) {
  if (!(arg instanceof pulumi_engine_pb.SetRootResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.SetRootResourceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_SetRootResourceResponse(buffer_arg) {
  return pulumi_engine_pb.SetRootResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Engine is an auxiliary service offered to language and resource provider plugins. Its main purpose today is
// to serve as a common logging endpoint, but it also serves as a state storage mechanism for language hosts
// that can't store their own global state.
var EngineService = exports.EngineService = {
  // Log logs a global message in the engine, including errors and warnings.
log: {
    path: '/pulumirpc.Engine/Log',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_pb.LogRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_LogRequest,
    requestDeserialize: deserialize_pulumirpc_LogRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // GetRootResource gets the URN of the root resource, the resource that should be the root of all
// otherwise-unparented resources.
getRootResource: {
    path: '/pulumirpc.Engine/GetRootResource',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_pb.GetRootResourceRequest,
    responseType: pulumi_engine_pb.GetRootResourceResponse,
    requestSerialize: serialize_pulumirpc_GetRootResourceRequest,
    requestDeserialize: deserialize_pulumirpc_GetRootResourceRequest,
    responseSerialize: serialize_pulumirpc_GetRootResourceResponse,
    responseDeserialize: deserialize_pulumirpc_GetRootResourceResponse,
  },
  // SetRootResource sets the URN of the root resource.
setRootResource: {
    path: '/pulumirpc.Engine/SetRootResource',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_pb.SetRootResourceRequest,
    responseType: pulumi_engine_pb.SetRootResourceResponse,
    requestSerialize: serialize_pulumirpc_SetRootResourceRequest,
    requestDeserialize: deserialize_pulumirpc_SetRootResourceRequest,
    responseSerialize: serialize_pulumirpc_SetRootResourceResponse,
    responseDeserialize: deserialize_pulumirpc_SetRootResourceResponse,
  },
};

exports.EngineClient = grpc.makeGenericClientConstructor(EngineService);
