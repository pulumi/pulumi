// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var provider_pb = require('./provider_pb.js');
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

function serialize_lumirpc_CheckRequest(arg) {
  if (!(arg instanceof provider_pb.CheckRequest)) {
    throw new Error('Expected argument of type lumirpc.CheckRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_CheckRequest(buffer_arg) {
  return provider_pb.CheckRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_CheckResponse(arg) {
  if (!(arg instanceof provider_pb.CheckResponse)) {
    throw new Error('Expected argument of type lumirpc.CheckResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_CheckResponse(buffer_arg) {
  return provider_pb.CheckResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_CreateRequest(arg) {
  if (!(arg instanceof provider_pb.CreateRequest)) {
    throw new Error('Expected argument of type lumirpc.CreateRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_CreateRequest(buffer_arg) {
  return provider_pb.CreateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_CreateResponse(arg) {
  if (!(arg instanceof provider_pb.CreateResponse)) {
    throw new Error('Expected argument of type lumirpc.CreateResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_CreateResponse(buffer_arg) {
  return provider_pb.CreateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_DeleteRequest(arg) {
  if (!(arg instanceof provider_pb.DeleteRequest)) {
    throw new Error('Expected argument of type lumirpc.DeleteRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_DeleteRequest(buffer_arg) {
  return provider_pb.DeleteRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_DiffRequest(arg) {
  if (!(arg instanceof provider_pb.DiffRequest)) {
    throw new Error('Expected argument of type lumirpc.DiffRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_DiffRequest(buffer_arg) {
  return provider_pb.DiffRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_DiffResponse(arg) {
  if (!(arg instanceof provider_pb.DiffResponse)) {
    throw new Error('Expected argument of type lumirpc.DiffResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_DiffResponse(buffer_arg) {
  return provider_pb.DiffResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_GetRequest(arg) {
  if (!(arg instanceof provider_pb.GetRequest)) {
    throw new Error('Expected argument of type lumirpc.GetRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_GetRequest(buffer_arg) {
  return provider_pb.GetRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_GetResponse(arg) {
  if (!(arg instanceof provider_pb.GetResponse)) {
    throw new Error('Expected argument of type lumirpc.GetResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_GetResponse(buffer_arg) {
  return provider_pb.GetResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_UpdateRequest(arg) {
  if (!(arg instanceof provider_pb.UpdateRequest)) {
    throw new Error('Expected argument of type lumirpc.UpdateRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_UpdateRequest(buffer_arg) {
  return provider_pb.UpdateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_lumirpc_UpdateResponse(arg) {
  if (!(arg instanceof provider_pb.UpdateResponse)) {
    throw new Error('Expected argument of type lumirpc.UpdateResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_lumirpc_UpdateResponse(buffer_arg) {
  return provider_pb.UpdateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// ResourceProvider is a service that understands how to create, read, update, or delete resources for types defined
// within a single package.  It is driven by the overall planning engine in response to resource diffs.
var ResourceProviderService = exports.ResourceProviderService = {
  // Check validates that the given property bag is valid for a resource of the given type.
  check: {
    path: '/lumirpc.ResourceProvider/Check',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.CheckRequest,
    responseType: provider_pb.CheckResponse,
    requestSerialize: serialize_lumirpc_CheckRequest,
    requestDeserialize: deserialize_lumirpc_CheckRequest,
    responseSerialize: serialize_lumirpc_CheckResponse,
    responseDeserialize: deserialize_lumirpc_CheckResponse,
  },
  // Diff checks what impacts a hypothetical update will have on the resource's properties.
  diff: {
    path: '/lumirpc.ResourceProvider/Diff',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.DiffRequest,
    responseType: provider_pb.DiffResponse,
    requestSerialize: serialize_lumirpc_DiffRequest,
    requestDeserialize: deserialize_lumirpc_DiffRequest,
    responseSerialize: serialize_lumirpc_DiffResponse,
    responseDeserialize: deserialize_lumirpc_DiffResponse,
  },
  // Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
  // must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
  create: {
    path: '/lumirpc.ResourceProvider/Create',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.CreateRequest,
    responseType: provider_pb.CreateResponse,
    requestSerialize: serialize_lumirpc_CreateRequest,
    requestDeserialize: deserialize_lumirpc_CreateRequest,
    responseSerialize: serialize_lumirpc_CreateResponse,
    responseDeserialize: deserialize_lumirpc_CreateResponse,
  },
  // Get reads the instance state identified by ID, returning a populated resource object, or nil if not found.
  get: {
    path: '/lumirpc.ResourceProvider/Get',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.GetRequest,
    responseType: provider_pb.GetResponse,
    requestSerialize: serialize_lumirpc_GetRequest,
    requestDeserialize: deserialize_lumirpc_GetRequest,
    responseSerialize: serialize_lumirpc_GetResponse,
    responseDeserialize: deserialize_lumirpc_GetResponse,
  },
  // Update updates an existing resource with new values.
  update: {
    path: '/lumirpc.ResourceProvider/Update',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.UpdateRequest,
    responseType: provider_pb.UpdateResponse,
    requestSerialize: serialize_lumirpc_UpdateRequest,
    requestDeserialize: deserialize_lumirpc_UpdateRequest,
    responseSerialize: serialize_lumirpc_UpdateResponse,
    responseDeserialize: deserialize_lumirpc_UpdateResponse,
  },
  // Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
  delete: {
    path: '/lumirpc.ResourceProvider/Delete',
    requestStream: false,
    responseStream: false,
    requestType: provider_pb.DeleteRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_lumirpc_DeleteRequest,
    requestDeserialize: deserialize_lumirpc_DeleteRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.ResourceProviderClient = grpc.makeGenericClientConstructor(ResourceProviderService);
