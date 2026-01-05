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

function serialize_pulumirpc_AnalyzerHandshakeRequest(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzerHandshakeRequest)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzerHandshakeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzerHandshakeRequest(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzerHandshakeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzerHandshakeResponse(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzerHandshakeResponse)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzerHandshakeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzerHandshakeResponse(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzerHandshakeResponse.deserializeBinary(new Uint8Array(buffer_arg));
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

function serialize_pulumirpc_AnalyzerStackConfigureRequest(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzerStackConfigureRequest)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzerStackConfigureRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzerStackConfigureRequest(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzerStackConfigureRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AnalyzerStackConfigureResponse(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.AnalyzerStackConfigureResponse)) {
    throw new Error('Expected argument of type pulumirpc.AnalyzerStackConfigureResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AnalyzerStackConfigureResponse(buffer_arg) {
  return pulumi_analyzer_pb.AnalyzerStackConfigureResponse.deserializeBinary(new Uint8Array(buffer_arg));
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

function serialize_pulumirpc_RemediateResponse(arg) {
  if (!(arg instanceof pulumi_analyzer_pb.RemediateResponse)) {
    throw new Error('Expected argument of type pulumirpc.RemediateResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RemediateResponse(buffer_arg) {
  return pulumi_analyzer_pb.RemediateResponse.deserializeBinary(new Uint8Array(buffer_arg));
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
  // Remediate optionally transforms a single resource object. This effectively rewrites
// a single resource object's properties instead of using what was generated by the program.
remediate: {
    path: '/pulumirpc.Analyzer/Remediate',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.AnalyzeRequest,
    responseType: pulumi_analyzer_pb.RemediateResponse,
    requestSerialize: serialize_pulumirpc_AnalyzeRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzeRequest,
    responseSerialize: serialize_pulumirpc_RemediateResponse,
    responseDeserialize: deserialize_pulumirpc_RemediateResponse,
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
  // `Handshake` is the first call made by the engine to an analyzer. It is used to pass the engine's address to the
// analyzer so that it may establish its own connections back, and to establish protocol configuration that will be
// used to communicate between the two parties.
handshake: {
    path: '/pulumirpc.Analyzer/Handshake',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.AnalyzerHandshakeRequest,
    responseType: pulumi_analyzer_pb.AnalyzerHandshakeResponse,
    requestSerialize: serialize_pulumirpc_AnalyzerHandshakeRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzerHandshakeRequest,
    responseSerialize: serialize_pulumirpc_AnalyzerHandshakeResponse,
    responseDeserialize: deserialize_pulumirpc_AnalyzerHandshakeResponse,
  },
  // `ConfigureStack` is always called if the engine is using the analyzer to analyze resources in a specific stack.
// This method is not always called, for example if the engine is just booting the analyzer up to call
// GetAnalyzerInfo.
configureStack: {
    path: '/pulumirpc.Analyzer/ConfigureStack',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_analyzer_pb.AnalyzerStackConfigureRequest,
    responseType: pulumi_analyzer_pb.AnalyzerStackConfigureResponse,
    requestSerialize: serialize_pulumirpc_AnalyzerStackConfigureRequest,
    requestDeserialize: deserialize_pulumirpc_AnalyzerStackConfigureRequest,
    responseSerialize: serialize_pulumirpc_AnalyzerStackConfigureResponse,
    responseDeserialize: deserialize_pulumirpc_AnalyzerStackConfigureResponse,
  },
  // Cancel signals the analyzer to gracefully shut down and abort any ongoing analysis operations.
// Operations aborted in this way will return an error. Since Cancel is advisory and non-blocking,
// it is up to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
cancel: {
    path: '/pulumirpc.Analyzer/Cancel',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.AnalyzerClient = grpc.makeGenericClientConstructor(AnalyzerService, 'Analyzer');
