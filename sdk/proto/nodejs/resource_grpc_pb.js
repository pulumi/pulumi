// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var resource_pb = require('./resource_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
var provider_pb = require('./provider_pb.js');

function serialize_pulumirpc_BeginRegisterResourceRequest(arg) {
  if (!(arg instanceof resource_pb.BeginRegisterResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.BeginRegisterResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_BeginRegisterResourceRequest(buffer_arg) {
  return resource_pb.BeginRegisterResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_BeginRegisterResourceResponse(arg) {
  if (!(arg instanceof resource_pb.BeginRegisterResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.BeginRegisterResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_BeginRegisterResourceResponse(buffer_arg) {
  return resource_pb.BeginRegisterResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_EndRegisterResourceRequest(arg) {
  if (!(arg instanceof resource_pb.EndRegisterResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.EndRegisterResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_EndRegisterResourceRequest(buffer_arg) {
  return resource_pb.EndRegisterResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_EndRegisterResourceResponse(arg) {
  if (!(arg instanceof resource_pb.EndRegisterResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.EndRegisterResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_EndRegisterResourceResponse(buffer_arg) {
  return resource_pb.EndRegisterResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
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
  beginRegisterResource: {
    path: '/pulumirpc.ResourceMonitor/BeginRegisterResource',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.BeginRegisterResourceRequest,
    responseType: resource_pb.BeginRegisterResourceResponse,
    requestSerialize: serialize_pulumirpc_BeginRegisterResourceRequest,
    requestDeserialize: deserialize_pulumirpc_BeginRegisterResourceRequest,
    responseSerialize: serialize_pulumirpc_BeginRegisterResourceResponse,
    responseDeserialize: deserialize_pulumirpc_BeginRegisterResourceResponse,
  },
  endRegisterResource: {
    path: '/pulumirpc.ResourceMonitor/EndRegisterResource',
    requestStream: false,
    responseStream: false,
    requestType: resource_pb.EndRegisterResourceRequest,
    responseType: resource_pb.EndRegisterResourceResponse,
    requestSerialize: serialize_pulumirpc_EndRegisterResourceRequest,
    requestDeserialize: deserialize_pulumirpc_EndRegisterResourceRequest,
    responseSerialize: serialize_pulumirpc_EndRegisterResourceResponse,
    responseDeserialize: deserialize_pulumirpc_EndRegisterResourceResponse,
  },
};

exports.ResourceMonitorClient = grpc.makeGenericClientConstructor(ResourceMonitorService);
