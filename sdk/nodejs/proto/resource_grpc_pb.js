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
var resource_pb = require('./resource_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
var provider_pb = require('./provider_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CallRequest(arg) {
  if (!(arg instanceof provider_pb.CallRequest)) {
    throw new Error('Expected argument of type pulumirpc.CallRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CallRequest(buffer_arg) {
  return provider_pb.CallRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CallResponse(arg) {
  if (!(arg instanceof provider_pb.CallResponse)) {
    throw new Error('Expected argument of type pulumirpc.CallResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CallResponse(buffer_arg) {
  return provider_pb.CallResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeRequest(arg) {
  if (!(arg instanceof provider_pb.InvokeRequest)) {
    throw new Error('Expected argument of type pulumirpc.InvokeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeRequest(buffer_arg) {
  return provider_pb.InvokeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeResponse(arg) {
  if (!(arg instanceof provider_pb.InvokeResponse)) {
    throw new Error('Expected argument of type pulumirpc.InvokeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeResponse(buffer_arg) {
  return provider_pb.InvokeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ReadResourceRequest(arg) {
  if (!(arg instanceof resource_pb.ReadResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.ReadResourceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ReadResourceRequest(buffer_arg) {
  return resource_pb.ReadResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ReadResourceResponse(arg) {
  if (!(arg instanceof resource_pb.ReadResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.ReadResourceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ReadResourceResponse(buffer_arg) {
  return resource_pb.ReadResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterResourceOutputsRequest(arg) {
  if (!(arg instanceof resource_pb.RegisterResourceOutputsRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterResourceOutputsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterResourceOutputsRequest(buffer_arg) {
  return resource_pb.RegisterResourceOutputsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterResourceRequest(arg) {
  if (!(arg instanceof resource_pb.RegisterResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterResourceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterResourceRequest(buffer_arg) {
  return resource_pb.RegisterResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterResourceResponse(arg) {
  if (!(arg instanceof resource_pb.RegisterResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.RegisterResourceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterResourceResponse(buffer_arg) {
  return resource_pb.RegisterResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_SupportsFeatureRequest(arg) {
  if (!(arg instanceof resource_pb.SupportsFeatureRequest)) {
    throw new Error('Expected argument of type pulumirpc.SupportsFeatureRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_SupportsFeatureRequest(buffer_arg) {
  return resource_pb.SupportsFeatureRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_SupportsFeatureResponse(arg) {
  if (!(arg instanceof resource_pb.SupportsFeatureResponse)) {
    throw new Error('Expected argument of type pulumirpc.SupportsFeatureResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_SupportsFeatureResponse(buffer_arg) {
  return resource_pb.SupportsFeatureResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// ResourceMonitor is the interface a source uses to talk back to the planning monitor orchestrating the execution.
var ResourceMonitorService = exports.ResourceMonitorService = {
  supportsFeature: {
    path: '/pulumirpc.ResourceMonitor/SupportsFeature',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.SupportsFeatureRequest,
    responseType: resource_pb.SupportsFeatureResponse,
    requestSerialize: serialize_pulumirpc_SupportsFeatureRequest,
    requestDeserialize: deserialize_pulumirpc_SupportsFeatureRequest,
    responseSerialize: serialize_pulumirpc_SupportsFeatureResponse,
    responseDeserialize: deserialize_pulumirpc_SupportsFeatureResponse,
  },
  invoke: {
    path: '/pulumirpc.ResourceMonitor/Invoke',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.InvokeRequest,
    responseType: provider_pb.InvokeResponse,
    requestSerialize: serialize_pulumirpc_InvokeRequest,
    requestDeserialize: deserialize_pulumirpc_InvokeRequest,
    responseSerialize: serialize_pulumirpc_InvokeResponse,
    responseDeserialize: deserialize_pulumirpc_InvokeResponse,
  },
  streamInvoke: {
    path: '/pulumirpc.ResourceMonitor/StreamInvoke',
    requestStream: false,
    responseStream: true,
    requestType: provider_pb.InvokeRequest,
    responseType: provider_pb.InvokeResponse,
    requestSerialize: serialize_pulumirpc_InvokeRequest,
    requestDeserialize: deserialize_pulumirpc_InvokeRequest,
    responseSerialize: serialize_pulumirpc_InvokeResponse,
    responseDeserialize: deserialize_pulumirpc_InvokeResponse,
  },
  call: {
    path: '/pulumirpc.ResourceMonitor/Call',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.CallRequest,
    responseType: provider_pb.CallResponse,
    requestSerialize: serialize_pulumirpc_CallRequest,
    requestDeserialize: deserialize_pulumirpc_CallRequest,
    responseSerialize: serialize_pulumirpc_CallResponse,
    responseDeserialize: deserialize_pulumirpc_CallResponse,
  },
  readResource: {
    path: '/pulumirpc.ResourceMonitor/ReadResource',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.ReadResourceRequest,
    responseType: resource_pb.ReadResourceResponse,
    requestSerialize: serialize_pulumirpc_ReadResourceRequest,
    requestDeserialize: deserialize_pulumirpc_ReadResourceRequest,
    responseSerialize: serialize_pulumirpc_ReadResourceResponse,
    responseDeserialize: deserialize_pulumirpc_ReadResourceResponse,
  },
  registerResource: {
    path: '/pulumirpc.ResourceMonitor/RegisterResource',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.RegisterResourceRequest,
    responseType: resource_pb.RegisterResourceResponse,
    requestSerialize: serialize_pulumirpc_RegisterResourceRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterResourceRequest,
    responseSerialize: serialize_pulumirpc_RegisterResourceResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterResourceResponse,
  },
  registerResourceOutputs: {
    path: '/pulumirpc.ResourceMonitor/RegisterResourceOutputs',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.RegisterResourceOutputsRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_RegisterResourceOutputsRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterResourceOutputsRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.ResourceMonitorClient = grpc.makeGenericClientConstructor(ResourceMonitorService);
