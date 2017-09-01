// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var languages_pb = require('./languages_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_lumirpc_NewResourceRequest(arg) {
  if (!(arg instanceof languages_pb.NewResourceRequest)) {
    throw new Error('Expected argument of type lumirpc.NewResourceRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_NewResourceRequest(buffer_arg) {
  return languages_pb.NewResourceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_NewResourceResponse(arg) {
  if (!(arg instanceof languages_pb.NewResourceResponse)) {
    throw new Error('Expected argument of type lumirpc.NewResourceResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_NewResourceResponse(buffer_arg) {
  return languages_pb.NewResourceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_RunRequest(arg) {
  if (!(arg instanceof languages_pb.RunRequest)) {
    throw new Error('Expected argument of type lumirpc.RunRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_RunRequest(buffer_arg) {
  return languages_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_RunResponse(arg) {
  if (!(arg instanceof languages_pb.RunResponse)) {
    throw new Error('Expected argument of type lumirpc.RunResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_RunResponse(buffer_arg) {
  return languages_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// LanguageRuntime is the interface that the planning monitor uses to drive execution of an interpreter responsible
// for confguring and creating resource objects.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
  run: {
    path: '/lumirpc.LanguageRuntime/Run',
    requestStream: false,
    responseStream: false,
    requestType: languages_pb.RunRequest,
    responseType: languages_pb.RunResponse,
    requestSerialize: serialize_lumirpc_RunRequest,
    requestDeserialize: deserialize_lumirpc_RunRequest,
    responseSerialize: serialize_lumirpc_RunResponse,
    responseDeserialize: deserialize_lumirpc_RunResponse,
  },
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService);
// ResourceMonitor is the interface a source uses to talk back to the planning monitor orchestrating the execution.
var ResourceMonitorService = exports.ResourceMonitorService = {
  newResource: {
    path: '/lumirpc.ResourceMonitor/NewResource',
    requestStream: false,
    responseStream: false,
    requestType: languages_pb.NewResourceRequest,
    responseType: languages_pb.NewResourceResponse,
    requestSerialize: serialize_lumirpc_NewResourceRequest,
    requestDeserialize: deserialize_lumirpc_NewResourceRequest,
    responseSerialize: serialize_lumirpc_NewResourceResponse,
    responseDeserialize: deserialize_lumirpc_NewResourceResponse,
  },
};

exports.ResourceMonitorClient = grpc.makeGenericClientConstructor(ResourceMonitorService);
