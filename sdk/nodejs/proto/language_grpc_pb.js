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
var language_pb = require('./language_pb.js');
var plugin_pb = require('./plugin_pb.js');
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

function serialize_pulumirpc_GetRequiredPluginsRequest(arg) {
  if (!(arg instanceof language_pb.GetRequiredPluginsRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsRequest(buffer_arg) {
  return language_pb.GetRequiredPluginsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPluginsResponse(arg) {
  if (!(arg instanceof language_pb.GetRequiredPluginsResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsResponse(buffer_arg) {
  return language_pb.GetRequiredPluginsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PluginInfo(arg) {
  if (!(arg instanceof plugin_pb.PluginInfo)) {
    throw new Error('Expected argument of type pulumirpc.PluginInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PluginInfo(buffer_arg) {
  return plugin_pb.PluginInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunRequest(arg) {
  if (!(arg instanceof language_pb.RunRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunRequest(buffer_arg) {
  return language_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunResponse(arg) {
  if (!(arg instanceof language_pb.RunResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunResponse(buffer_arg) {
  return language_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// LanguageRuntime is the interface that the planning monitor uses to drive execution of an interpreter responsible
// for confguring and creating resource objects.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
  // GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
  getRequiredPlugins: {
    path: '/pulumirpc.LanguageRuntime/GetRequiredPlugins',
    requestStream: false,
    responseStream: false,
    requestType: language_pb.GetRequiredPluginsRequest,
    responseType: language_pb.GetRequiredPluginsResponse,
    requestSerialize: serialize_pulumirpc_GetRequiredPluginsRequest,
    requestDeserialize: deserialize_pulumirpc_GetRequiredPluginsRequest,
    responseSerialize: serialize_pulumirpc_GetRequiredPluginsResponse,
    responseDeserialize: deserialize_pulumirpc_GetRequiredPluginsResponse,
  },
  // Run executes a program and returns its result.
  run: {
    path: '/pulumirpc.LanguageRuntime/Run',
    requestStream: false,
    responseStream: false,
    requestType: language_pb.RunRequest,
    responseType: language_pb.RunResponse,
    requestSerialize: serialize_pulumirpc_RunRequest,
    requestDeserialize: deserialize_pulumirpc_RunRequest,
    responseSerialize: serialize_pulumirpc_RunResponse,
    responseDeserialize: deserialize_pulumirpc_RunResponse,
  },
  // GetPluginInfo returns generic information about this plugin, like its version.
  getPluginInfo: {
    path: '/pulumirpc.LanguageRuntime/GetPluginInfo',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: plugin_pb.PluginInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_PluginInfo,
    responseDeserialize: deserialize_pulumirpc_PluginInfo,
  },
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService);
