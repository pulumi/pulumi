// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var resource_pb = require('./resource_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
var provider_pb = require('./provider_pb.js');

function serialize_pulumirpc_CompleteResourceRequest(arg) {
  if (!(arg instanceof resource_pb.CompleteResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.CompleteResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_CompleteResourceRequest(buffer_arg) {
  return resource_pb.CompleteResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CompleteResourceResponse(arg) {
  if (!(arg instanceof resource_pb.CompleteResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.CompleteResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_CompleteResourceResponse(buffer_arg) {
  return resource_pb.CompleteResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeRequest(arg) {
  if (!(arg instanceof provider_pb.InvokeRequest)) {
    throw new Error('Expected argument of type pulumirpc.InvokeRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeRequest(buffer_arg) {
  return provider_pb.InvokeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeResponse(arg) {
  if (!(arg instanceof provider_pb.InvokeResponse)) {
    throw new Error('Expected argument of type pulumirpc.InvokeResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeResponse(buffer_arg) {
  return provider_pb.InvokeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterResourceRequest(arg) {
  if (!(arg instanceof resource_pb.RegisterResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterResourceRequest(buffer_arg) {
  return resource_pb.RegisterResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterResourceResponse(arg) {
  if (!(arg instanceof resource_pb.RegisterResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.RegisterResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterResourceResponse(buffer_arg) {
  return resource_pb.RegisterResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// ResourceMonitor is the interface a source uses to talk back to the planning monitor orchestrating the execution.
var ResourceMonitorService = exports.ResourceMonitorService = {
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
  completeResource: {
    path: '/pulumirpc.ResourceMonitor/CompleteResource',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.CompleteResourceRequest,
    responseType: resource_pb.CompleteResourceResponse,
    requestSerialize: serialize_pulumirpc_CompleteResourceRequest,
    requestDeserialize: deserialize_pulumirpc_CompleteResourceRequest,
    responseSerialize: serialize_pulumirpc_CompleteResourceResponse,
    responseDeserialize: deserialize_pulumirpc_CompleteResourceResponse,
  },
};

exports.ResourceMonitorClient = grpc.makeGenericClientConstructor(ResourceMonitorService);
