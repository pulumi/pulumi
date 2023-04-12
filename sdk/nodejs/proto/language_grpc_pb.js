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
var pulumi_language_pb = require('./language_pb.js');
var pulumi_plugin_pb = require('./plugin_pb.js');
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

function serialize_pulumirpc_AboutResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.AboutResponse)) {
    throw new Error('Expected argument of type pulumirpc.AboutResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AboutResponse(buffer_arg) {
  return pulumi_language_pb.AboutResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetProgramDependenciesRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GetProgramDependenciesRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetProgramDependenciesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetProgramDependenciesRequest(buffer_arg) {
  return pulumi_language_pb.GetProgramDependenciesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetProgramDependenciesResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GetProgramDependenciesResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetProgramDependenciesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetProgramDependenciesResponse(buffer_arg) {
  return pulumi_language_pb.GetProgramDependenciesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPluginsRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPluginsRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsRequest(buffer_arg) {
  return pulumi_language_pb.GetRequiredPluginsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPluginsResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPluginsResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsResponse(buffer_arg) {
  return pulumi_language_pb.GetRequiredPluginsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InstallDependenciesRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.InstallDependenciesRequest)) {
    throw new Error('Expected argument of type pulumirpc.InstallDependenciesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InstallDependenciesRequest(buffer_arg) {
  return pulumi_language_pb.InstallDependenciesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InstallDependenciesResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.InstallDependenciesResponse)) {
    throw new Error('Expected argument of type pulumirpc.InstallDependenciesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InstallDependenciesResponse(buffer_arg) {
  return pulumi_language_pb.InstallDependenciesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PluginInfo(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PluginInfo)) {
    throw new Error('Expected argument of type pulumirpc.PluginInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PluginInfo(buffer_arg) {
  return pulumi_plugin_pb.PluginInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunPluginRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.RunPluginRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunPluginRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunPluginRequest(buffer_arg) {
  return pulumi_language_pb.RunPluginRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunPluginResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.RunPluginResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunPluginResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunPluginResponse(buffer_arg) {
  return pulumi_language_pb.RunPluginResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.RunRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunRequest(buffer_arg) {
  return pulumi_language_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.RunResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunResponse(buffer_arg) {
  return pulumi_language_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// LanguageRuntime is the interface that the planning monitor uses to drive execution of an interpreter responsible
// for confguring and creating resource objects.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
  // GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
getRequiredPlugins: {
    path: '/pulumirpc.LanguageRuntime/GetRequiredPlugins',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GetRequiredPluginsRequest,
    responseType: pulumi_language_pb.GetRequiredPluginsResponse,
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
    requestType: pulumi_language_pb.RunRequest,
    responseType: pulumi_language_pb.RunResponse,
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
    responseType: pulumi_plugin_pb.PluginInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_PluginInfo,
    responseDeserialize: deserialize_pulumirpc_PluginInfo,
  },
  // InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
installDependencies: {
    path: '/pulumirpc.LanguageRuntime/InstallDependencies',
    requestStream: false,
    responseStream: true,
    requestType: pulumi_language_pb.InstallDependenciesRequest,
    responseType: pulumi_language_pb.InstallDependenciesResponse,
    requestSerialize: serialize_pulumirpc_InstallDependenciesRequest,
    requestDeserialize: deserialize_pulumirpc_InstallDependenciesRequest,
    responseSerialize: serialize_pulumirpc_InstallDependenciesResponse,
    responseDeserialize: deserialize_pulumirpc_InstallDependenciesResponse,
  },
  // About returns information about the runtime for this language.
about: {
    path: '/pulumirpc.LanguageRuntime/About',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_language_pb.AboutResponse,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_AboutResponse,
    responseDeserialize: deserialize_pulumirpc_AboutResponse,
  },
  // GetProgramDependencies returns the set of dependencies required by the program.
getProgramDependencies: {
    path: '/pulumirpc.LanguageRuntime/GetProgramDependencies',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GetProgramDependenciesRequest,
    responseType: pulumi_language_pb.GetProgramDependenciesResponse,
    requestSerialize: serialize_pulumirpc_GetProgramDependenciesRequest,
    requestDeserialize: deserialize_pulumirpc_GetProgramDependenciesRequest,
    responseSerialize: serialize_pulumirpc_GetProgramDependenciesResponse,
    responseDeserialize: deserialize_pulumirpc_GetProgramDependenciesResponse,
  },
  // RunPlugin executes a plugin program and returns its result asynchronously.
runPlugin: {
    path: '/pulumirpc.LanguageRuntime/RunPlugin',
    requestStream: false,
    responseStream: true,
    requestType: pulumi_language_pb.RunPluginRequest,
    responseType: pulumi_language_pb.RunPluginResponse,
    requestSerialize: serialize_pulumirpc_RunPluginRequest,
    requestDeserialize: deserialize_pulumirpc_RunPluginRequest,
    responseSerialize: serialize_pulumirpc_RunPluginResponse,
    responseDeserialize: deserialize_pulumirpc_RunPluginResponse,
  },
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService);
