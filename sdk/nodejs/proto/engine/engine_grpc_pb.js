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
var grpc = require('grpc');
var pulumi_engine_engine_pb = require('../../pulumi/engine/engine_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');

function serialize_pulumirpc_engine_GetLanguageTestsRequest(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.GetLanguageTestsRequest)) {
    throw new Error('Expected argument of type pulumirpc.engine.GetLanguageTestsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_GetLanguageTestsRequest(buffer_arg) {
  return pulumi_engine_engine_pb.GetLanguageTestsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_GetLanguageTestsResponse(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.GetLanguageTestsResponse)) {
    throw new Error('Expected argument of type pulumirpc.engine.GetLanguageTestsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_GetLanguageTestsResponse(buffer_arg) {
  return pulumi_engine_engine_pb.GetLanguageTestsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_PrepareLanguageTestsRequest(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.PrepareLanguageTestsRequest)) {
    throw new Error('Expected argument of type pulumirpc.engine.PrepareLanguageTestsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_PrepareLanguageTestsRequest(buffer_arg) {
  return pulumi_engine_engine_pb.PrepareLanguageTestsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_PrepareLanguageTestsResponse(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.PrepareLanguageTestsResponse)) {
    throw new Error('Expected argument of type pulumirpc.engine.PrepareLanguageTestsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_PrepareLanguageTestsResponse(buffer_arg) {
  return pulumi_engine_engine_pb.PrepareLanguageTestsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_RunLanguageTestRequest(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.RunLanguageTestRequest)) {
    throw new Error('Expected argument of type pulumirpc.engine.RunLanguageTestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_RunLanguageTestRequest(buffer_arg) {
  return pulumi_engine_engine_pb.RunLanguageTestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_RunLanguageTestResponse(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.RunLanguageTestResponse)) {
    throw new Error('Expected argument of type pulumirpc.engine.RunLanguageTestResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_RunLanguageTestResponse(buffer_arg) {
  return pulumi_engine_engine_pb.RunLanguageTestResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// EngineInterface is the interface to the core pulumi engine, it is used by both the CLI and the automation
// API to run all core commands. This is _highly_ experimental and currently subject to breaking changes without warning.
var EngineService = exports.EngineService = {
  // GetLanguageTests returns a list of all the language tests.
getLanguageTests: {
    path: '/pulumirpc.engine.Engine/GetLanguageTests',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_engine_pb.GetLanguageTestsRequest,
    responseType: pulumi_engine_engine_pb.GetLanguageTestsResponse,
    requestSerialize: serialize_pulumirpc_engine_GetLanguageTestsRequest,
    requestDeserialize: deserialize_pulumirpc_engine_GetLanguageTestsRequest,
    responseSerialize: serialize_pulumirpc_engine_GetLanguageTestsResponse,
    responseDeserialize: deserialize_pulumirpc_engine_GetLanguageTestsResponse,
  },
  // PrepareLanguageTests prepares the engine to run language tests. It sets up a stable artifacts folder
// (which should be .gitignore'd) and fills it with the core SDK artifact.
prepareLanguageTests: {
    path: '/pulumirpc.engine.Engine/PrepareLanguageTests',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_engine_pb.PrepareLanguageTestsRequest,
    responseType: pulumi_engine_engine_pb.PrepareLanguageTestsResponse,
    requestSerialize: serialize_pulumirpc_engine_PrepareLanguageTestsRequest,
    requestDeserialize: deserialize_pulumirpc_engine_PrepareLanguageTestsRequest,
    responseSerialize: serialize_pulumirpc_engine_PrepareLanguageTestsResponse,
    responseDeserialize: deserialize_pulumirpc_engine_PrepareLanguageTestsResponse,
  },
  // RunLanguageTest runs a single test of the language plugin.
runLanguageTest: {
    path: '/pulumirpc.engine.Engine/RunLanguageTest',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_engine_pb.RunLanguageTestRequest,
    responseType: pulumi_engine_engine_pb.RunLanguageTestResponse,
    requestSerialize: serialize_pulumirpc_engine_RunLanguageTestRequest,
    requestDeserialize: deserialize_pulumirpc_engine_RunLanguageTestRequest,
    responseSerialize: serialize_pulumirpc_engine_RunLanguageTestResponse,
    responseDeserialize: deserialize_pulumirpc_engine_RunLanguageTestResponse,
  },
};

exports.EngineClient = grpc.makeGenericClientConstructor(EngineService);
