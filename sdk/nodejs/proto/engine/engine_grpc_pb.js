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
var pulumi_engine_engine_pb = require('../../pulumi/engine/engine_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var pulumi_plugin_pb = require('../../pulumi/plugin_pb.js');
var pulumi_language_pb = require('../../pulumi/language_pb.js');

function serialize_pulumirpc_engine_AboutRequest(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.AboutRequest)) {
    throw new Error('Expected argument of type pulumirpc.engine.AboutRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_AboutRequest(buffer_arg) {
  return pulumi_engine_engine_pb.AboutRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_engine_AboutResponse(arg) {
  if (!(arg instanceof pulumi_engine_engine_pb.AboutResponse)) {
    throw new Error('Expected argument of type pulumirpc.engine.AboutResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_engine_AboutResponse(buffer_arg) {
  return pulumi_engine_engine_pb.AboutResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// EngineInterface is the interface to the core pulumi engine, it is used by both the CLI and the automation
// API to run all core commands. This is _highly_ experimental and currently subject to breaking changes without warning.
var EngineService = exports.EngineService = {
  // About returns information about the pulumi engine, and the current workspace.
about: {
    path: '/pulumirpc.engine.Engine/About',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_engine_engine_pb.AboutRequest,
    responseType: pulumi_engine_engine_pb.AboutResponse,
    requestSerialize: serialize_pulumirpc_engine_AboutRequest,
    requestDeserialize: deserialize_pulumirpc_engine_AboutRequest,
    responseSerialize: serialize_pulumirpc_engine_AboutResponse,
    responseDeserialize: deserialize_pulumirpc_engine_AboutResponse,
  },
};

exports.EngineClient = grpc.makeGenericClientConstructor(EngineService);
