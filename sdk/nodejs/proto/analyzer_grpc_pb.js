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
var pulumi_analyzer_pb = require('./analyzer_pb.js');
var pulumi_plugin_pb = require('./plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzeRequest(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzeRequest)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzeRequest(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzeResponse(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzeResponse)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzeResponse(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzeStackRequest(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzeStackRequest)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzeStackRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzeStackRequest(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzeStackRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzerInfo(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzerInfo)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzerInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzerInfo(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzerInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConfigureAnalyzerRequest(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.ConfigureAnalyzerRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConfigureAnalyzerRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConfigureAnalyzerRequest(buffer_arg) {
  return pulumi_analyzer_pb.ConfigureAnalyzerRequest.deserializeBinary(new Uint8Array(buffer_arg));
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


// Analyzer provides a pluggable interface for checking resource definitions against some number of
// resource policies. It is intentionally open-ended, allowing for implementations that check
// everything from raw resource definitions to entire projects/stacks/snapshots for arbitrary
// issues -- style, policy, correctness, security, and so on.
var AnalyzerService = exports.AnalyzerService = {
  // Analyze analyzes a single resource object, and returns any errors that it finds.
// Called with the "inputs" to the resource, before it is updated.
analyze: {
    path: '/pulumirpc.Analyzer/Analyze',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.AnalyzeRequest,
    responseType: pulumi_analyzer_pb.AnalyzeResponse,
    requestSerialize: serialize_pulumirpc_AnalyzeRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzeRequest,
    responseSerialize: serialize_pulumirpc_AnalyzeResponse,
    responseDeserialize: deserialize_pulumirpc_AnalyzeResponse,
  },
  // AnalyzeStack analyzes all resources within a stack, at the end of a successful
// preview or update. The provided resources are the "outputs", after any mutations
// have taken place.
analyzeStack: {
    path: '/pulumirpc.Analyzer/AnalyzeStack',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.AnalyzeStackRequest,
    responseType: pulumi_analyzer_pb.AnalyzeResponse,
    requestSerialize: serialize_pulumirpc_AnalyzeStackRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzeStackRequest,
    responseSerialize: serialize_pulumirpc_AnalyzeResponse,
    responseDeserialize: deserialize_pulumirpc_AnalyzeResponse,
  },
  // GetAnalyzerInfo returns metadata about the analyzer (e.g., list of policies contained).
getAnalyzerInfo: {
    path: '/pulumirpc.Analyzer/GetAnalyzerInfo',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_analyzer_pb.AnalyzerInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_AnalyzerInfo,
    responseDeserialize: deserialize_pulumirpc_AnalyzerInfo,
  },
  // GetPluginInfo returns generic information about this plugin, like its version.
getPluginInfo: {
    path: '/pulumirpc.Analyzer/GetPluginInfo',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_plugin_pb.PluginInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_PluginInfo,
    responseDeserialize: deserialize_pulumirpc_PluginInfo,
  },
  // Configure configures the analyzer, passing configuration properties for each policy.
configure: {
    path: '/pulumirpc.Analyzer/Configure',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.ConfigureAnalyzerRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_ConfigureAnalyzerRequest,
    requestDeserialize: deserialize_pulumirpc_ConfigureAnalyzerRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.AnalyzerClient = grpc.makeGenericClientConstructor(AnalyzerService);
