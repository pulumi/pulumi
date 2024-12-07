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

function serialize_codegen_PackageDescriptor(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.PackageDescriptor)) {
    throw new Error('Expected argument of type codegen.PackageDescriptor');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_PackageDescriptor(buffer_arg) {
  return pulumi_codegen_loader_pb.PackageDescriptor.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_PackageDescriptorMember(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.PackageDescriptorMember)) {
    throw new Error('Expected argument of type codegen.PackageDescriptorMember');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_PackageDescriptorMember(buffer_arg) {
  return pulumi_codegen_loader_pb.PackageDescriptorMember.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_PackageSpec(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.PackageSpec)) {
    throw new Error('Expected argument of type codegen.PackageSpec');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_PackageSpec(buffer_arg) {
  return pulumi_codegen_loader_pb.PackageSpec.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_ResourceSpec(arg) {
  if (!(arg instanceof pulumi_codegen_loader_pb.ResourceSpec)) {
    throw new Error('Expected argument of type codegen.ResourceSpec');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_ResourceSpec(buffer_arg) {
  return pulumi_codegen_loader_pb.ResourceSpec.deserializeBinary(new Uint8Array(buffer_arg));
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
  // GetPackageSpec returns information about a package, such as its name, version, description, and repository.
getPackageSpec: {
    path: '/codegen.Loader/GetPackageSpec',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.PackageDescriptor,
    responseType: pulumi_codegen_loader_pb.PackageSpec,
    requestSerialize: serialize_codegen_PackageDescriptor,
    requestDeserialize: deserialize_codegen_PackageDescriptor,
    responseSerialize: serialize_codegen_PackageSpec,
    responseDeserialize: deserialize_codegen_PackageSpec,
  },
  // GetResourceSpec returns information about a resource in a package, such as its name, description, and properties.
getResourceSpec: {
    path: '/codegen.Loader/GetResourceSpec',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_loader_pb.PackageDescriptorMember,
    responseType: pulumi_codegen_loader_pb.ResourceSpec,
    requestSerialize: serialize_codegen_PackageDescriptorMember,
    requestDeserialize: deserialize_codegen_PackageDescriptorMember,
    responseSerialize: serialize_codegen_ResourceSpec,
    responseDeserialize: deserialize_codegen_ResourceSpec,
  },
};

exports.LoaderClient = grpc.makeGenericClientConstructor(LoaderService);
