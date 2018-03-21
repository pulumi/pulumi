// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var languages_pb = require('./languages_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
var provider_pb = require('./provider_pb.js');

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

function serialize_pulumirpc_NewResourceRequest(arg) {
  if (!(arg instanceof languages_pb.NewResourceRequest)) {
    throw new Error('Expected argument of type pulumirpc.NewResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_NewResourceRequest(buffer_arg) {
  return languages_pb.NewResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_NewResourceResponse(arg) {
  if (!(arg instanceof languages_pb.NewResourceResponse)) {
    throw new Error('Expected argument of type pulumirpc.NewResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_NewResourceResponse(buffer_arg) {
  return languages_pb.NewResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunRequest(arg) {
  if (!(arg instanceof languages_pb.RunRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RunRequest(buffer_arg) {
  return languages_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunResponse(arg) {
  if (!(arg instanceof languages_pb.RunResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RunResponse(buffer_arg) {
  return languages_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// LanguageRuntime is the interface that the planning monitor uses to drive execution of an interpreter responsible
// for confguring and creating resource objects.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
  run: {
    path: '/pulumirpc.LanguageRuntime/Run',
    requestStream: false,
    responseStream: false,
    requestType: languages_pb.RunRequest,
    responseType: languages_pb.RunResponse,
    requestSerialize: serialize_pulumirpc_RunRequest,
    requestDeserialize: deserialize_pulumirpc_RunRequest,
    responseSerialize: serialize_pulumirpc_RunResponse,
    responseDeserialize: deserialize_pulumirpc_RunResponse,
  },
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService);
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
  newResource: {
    path: '/pulumirpc.ResourceMonitor/NewResource',
    requestStream: false,
    responseStream: false,
    requestType: languages_pb.NewResourceRequest,
    responseType: languages_pb.NewResourceResponse,
    requestSerialize: serialize_pulumirpc_NewResourceRequest,
    requestDeserialize: deserialize_pulumirpc_NewResourceRequest,
    responseSerialize: serialize_pulumirpc_NewResourceResponse,
    responseDeserialize: deserialize_pulumirpc_NewResourceResponse,
  },
};

exports.ResourceMonitorClient = grpc.makeGenericClientConstructor(ResourceMonitorService);
