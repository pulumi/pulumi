// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var language_pb = require('./language_pb.js');

function serialize_pulumirpc_RunRequest(arg) {
  if (!(arg instanceof language_pb.RunRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RunRequest(buffer_arg) {
  return language_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunResponse(arg) {
  if (!(arg instanceof language_pb.RunResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_RunResponse(buffer_arg) {
  return language_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// LanguageRuntime is the interface that the planning monitor uses to drive execution of an interpreter responsible
// for confguring and creating resource objects.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
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
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService);
