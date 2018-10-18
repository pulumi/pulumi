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
var analyzer_pb = require('./analyzer_pb.js');
var plugin_pb = require('./plugin_pb.js');
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
  if (!(arg instanceof analyzer_pb.AnalyzeRequest)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzeRequest(buffer_arg) {
  return analyzer_pb.AnalyzeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzeResponse(arg) {
  if (!(arg instanceof analyzer_pb.AnalyzeResponse)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzeResponse(buffer_arg) {
  return analyzer_pb.AnalyzeResponse.deserializeBinary(new Uint8Array(buffer_arg));
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


// Analyzer is a pluggable service that checks entire projects/stacks/snapshots, and/or individual resources,
// for arbitrary issues.  These might be style, policy, correctness, security, or performance related.
var AnalyzerService = exports.AnalyzerService = {
  // Analyze analyzes a single resource object, and returns any errors that it finds.
  analyze: {
    path: '/pulumirpc.Analyzer/Analyze',
    requestStream: false,
    responseStream: false,
    requestType: analyzer_pb.AnalyzeRequest,
    responseType: analyzer_pb.AnalyzeResponse,
    requestSerialize: serialize_pulumirpc_AnalyzeRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzeRequest,
    responseSerialize: serialize_pulumirpc_AnalyzeResponse,
    responseDeserialize: deserialize_pulumirpc_AnalyzeResponse,
  },
  // GetPluginInfo returns generic information about this plugin, like its version.
  getPluginInfo: {
    path: '/pulumirpc.Analyzer/GetPluginInfo',
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

exports.AnalyzerClient = grpc.makeGenericClientConstructor(AnalyzerService);
