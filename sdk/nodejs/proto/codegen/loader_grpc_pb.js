// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2023, Pulumi Corporation.
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
var pulumi_codegen_loader_pb = require('../codegen/loader_pb.js');
var pulumi_codegen_schema_schema_pb = require('../codegen/schema/schema_pb.js');

function serialize_codegen_GetPartialSchemaPartRequest(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.GetPartialSchemaPartRequest)) {
    throw new Error('Expected argument of type codegen.GetPartialSchemaPartRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetPartialSchemaPartRequest(buffer_arg) {
  return pulumi_codegen_loader_pb.GetPartialSchemaPartRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetPartialSchemaRequest(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.GetPartialSchemaRequest)) {
    throw new Error('Expected argument of type codegen.GetPartialSchemaRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetPartialSchemaRequest(buffer_arg) {
  return pulumi_codegen_loader_pb.GetPartialSchemaRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetSchemaPartRequest(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.GetSchemaPartRequest)) {
    throw new Error('Expected argument of type codegen.GetSchemaPartRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetSchemaPartRequest(buffer_arg) {
  return pulumi_codegen_loader_pb.GetSchemaPartRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetSchemaRequest(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.GetSchemaRequest)) {
    throw new Error('Expected argument of type codegen.GetSchemaRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetSchemaRequest(buffer_arg) {
  return pulumi_codegen_loader_pb.GetSchemaRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetSchemaResponse(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.GetSchemaResponse)) {
    throw new Error('Expected argument of type codegen.GetSchemaResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetSchemaResponse(buffer_arg) {
  return pulumi_codegen_loader_pb.GetSchemaResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_codegen_schema_List(arg) {
  if (!(arg instanceof pulumi_codegen_schema_schema_pb.List)) {
    throw new Error('Expected argument of type pulumirpc.codegen.schema.List');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_codegen_schema_List(buffer_arg) {
  return pulumi_codegen_schema_schema_pb.List.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_codegen_schema_PackageInfo(arg) {
  if (!(arg instanceof pulumi_codegen_schema_schema_pb.PackageInfo)) {
    throw new Error('Expected argument of type pulumirpc.codegen.schema.PackageInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_codegen_schema_PackageInfo(buffer_arg) {
  return pulumi_codegen_schema_schema_pb.PackageInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_codegen_schema_Resource(arg) {
  if (!(arg instanceof pulumi_codegen_schema_schema_pb.Resource)) {
    throw new Error('Expected argument of type pulumirpc.codegen.schema.Resource');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_codegen_schema_Resource(buffer_arg) {
  return pulumi_codegen_schema_schema_pb.Resource.deserializeBinary(new Uint8Array(buffer_arg));
}


// Loader is a service for getting schemas from the Pulumi engine for use in code generators and other tools.
// This is currently unstable and experimental.
var LoaderService = exports.LoaderService = {
  // GetSchema tries to find a schema for the given package and version.
getSchema: {
    path: '/codegen.Loader/GetSchema',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetSchemaRequest,
    responseType: pulumi_codegen_loader_pb.GetSchemaResponse,
    requestSerialize: serialize_codegen_GetSchemaRequest,
    requestDeserialize: deserialize_codegen_GetSchemaRequest,
    responseSerialize: serialize_codegen_GetSchemaResponse,
    responseDeserialize: deserialize_codegen_GetSchemaResponse,
  },
  getPackageInfo: {
    path: '/codegen.Loader/GetPackageInfo',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetSchemaRequest,
    responseType: pulumi_codegen_schema_schema_pb.PackageInfo,
    requestSerialize: serialize_codegen_GetSchemaRequest,
    requestDeserialize: deserialize_codegen_GetSchemaRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_PackageInfo,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_PackageInfo,
  },
  getResources: {
    path: '/codegen.Loader/GetResources',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetSchemaRequest,
    responseType: pulumi_codegen_schema_schema_pb.List,
    requestSerialize: serialize_codegen_GetSchemaRequest,
    requestDeserialize: deserialize_codegen_GetSchemaRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_List,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_List,
  },
  getResource: {
    path: '/codegen.Loader/GetResource',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetSchemaPartRequest,
    responseType: pulumi_codegen_schema_schema_pb.Resource,
    requestSerialize: serialize_codegen_GetSchemaPartRequest,
    requestDeserialize: deserialize_codegen_GetSchemaPartRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_Resource,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_Resource,
  },
};

exports.LoaderClient = grpc.makeGenericClientConstructor(LoaderService);
// PartialLoader is a service a provider can implement to allow the engine to only load partial parts of the schema.
// This uses many of the same response message as the engine Loader service, but takes different requests.
var PartialLoaderService = exports.PartialLoaderService = {
  getPackageInfo: {
    path: '/codegen.PartialLoader/GetPackageInfo',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetPartialSchemaRequest,
    responseType: pulumi_codegen_schema_schema_pb.PackageInfo,
    requestSerialize: serialize_codegen_GetPartialSchemaRequest,
    requestDeserialize: deserialize_codegen_GetPartialSchemaRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_PackageInfo,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_PackageInfo,
  },
  getResources: {
    path: '/codegen.PartialLoader/GetResources',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetPartialSchemaRequest,
    responseType: pulumi_codegen_schema_schema_pb.List,
    requestSerialize: serialize_codegen_GetPartialSchemaRequest,
    requestDeserialize: deserialize_codegen_GetPartialSchemaRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_List,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_List,
  },
  getResource: {
    path: '/codegen.PartialLoader/GetResource',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.GetPartialSchemaPartRequest,
    responseType: pulumi_codegen_schema_schema_pb.Resource,
    requestSerialize: serialize_codegen_GetPartialSchemaPartRequest,
    requestDeserialize: deserialize_codegen_GetPartialSchemaPartRequest,
    responseSerialize: serialize_pulumirpc_codegen_schema_Resource,
    responseDeserialize: deserialize_pulumirpc_codegen_schema_Resource,
  },
};

exports.PartialLoaderClient = grpc.makeGenericClientConstructor(PartialLoaderService);
