// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var engine_pb = require('./engine_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_google_protobuf_Value(arg) {
  if (!(arg instanceof google_protobuf_struct_pb.Value)) {
    throw new Error('Expected argument of type google.protobuf.Value');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_google_protobuf_Value(buffer_arg) {
  return google_protobuf_struct_pb.Value.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_LogRequest(arg) {
  if (!(arg instanceof engine_pb.LogRequest)) {
    throw new Error('Expected argument of type lumirpc.LogRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_LogRequest(buffer_arg) {
  return engine_pb.LogRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_ReadLocationRequest(arg) {
  if (!(arg instanceof engine_pb.ReadLocationRequest)) {
    throw new Error('Expected argument of type lumirpc.ReadLocationRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_ReadLocationRequest(buffer_arg) {
  return engine_pb.ReadLocationRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_ReadLocationsRequest(arg) {
  if (!(arg instanceof engine_pb.ReadLocationsRequest)) {
    throw new Error('Expected argument of type lumirpc.ReadLocationsRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_ReadLocationsRequest(buffer_arg) {
  return engine_pb.ReadLocationsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_ReadLocationsResponse(arg) {
  if (!(arg instanceof engine_pb.ReadLocationsResponse)) {
    throw new Error('Expected argument of type lumirpc.ReadLocationsResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_ReadLocationsResponse(buffer_arg) {
  return engine_pb.ReadLocationsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Engine is an interface into the core engine responsible for orchestrating resource operations.
var EngineService = exports.EngineService = {
  // Log logs a global message in the engine, including errors and warnings.
  log: {
    path: '/lumirpc.Engine/Log',
    requestStream: false,
    responseStream: false,
    requestType: engine_pb.LogRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_lumirpc_LogRequest,
    requestDeserialize: deserialize_lumirpc_LogRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // ReadLocation reads the value from a location identified by a token in the current program.
  readLocation: {
    path: '/lumirpc.Engine/ReadLocation',
    requestStream: false,
    responseStream: false,
    requestType: engine_pb.ReadLocationRequest,
    responseType: google_protobuf_struct_pb.Value,
    requestSerialize: serialize_lumirpc_ReadLocationRequest,
    requestDeserialize: deserialize_lumirpc_ReadLocationRequest,
    responseSerialize: serialize_google_protobuf_Value,
    responseDeserialize: deserialize_google_protobuf_Value,
  },
  // ReadLocations reads a list of static or module variables from a single parent token.
  readLocations: {
    path: '/lumirpc.Engine/ReadLocations',
    requestStream: false,
    responseStream: false,
    requestType: engine_pb.ReadLocationsRequest,
    responseType: engine_pb.ReadLocationsResponse,
    requestSerialize: serialize_lumirpc_ReadLocationsRequest,
    requestDeserialize: deserialize_lumirpc_ReadLocationsRequest,
    responseSerialize: serialize_lumirpc_ReadLocationsResponse,
    responseDeserialize: deserialize_lumirpc_ReadLocationsResponse,
  },
};

exports.EngineClient = grpc.makeGenericClientConstructor(EngineService);
