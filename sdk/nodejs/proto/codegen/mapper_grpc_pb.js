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
var pulumi_codegen_mapper_pb = require('../codegen/mapper_pb.js');

function serialize_codegen_GetMappingRequest(arg) {
  if (!(arg instanceof pulumi_codegen_mapper_pb.GetMappingRequest)) {
    throw new Error('Expected argument of type codegen.GetMappingRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetMappingRequest(buffer_arg) {
  return pulumi_codegen_mapper_pb.GetMappingRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetMappingResponse(arg) {
  if (!(arg instanceof pulumi_codegen_mapper_pb.GetMappingResponse)) {
    throw new Error('Expected argument of type codegen.GetMappingResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetMappingResponse(buffer_arg) {
  return pulumi_codegen_mapper_pb.GetMappingResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Mapper is a service for getting mappings from other ecosystems to Pulumi.
// This is currently unstable and experimental.
var MapperService = exports.MapperService = {
  // GetMapping tries to find a mapping for the given provider.
getMapping: {
    path: '/codegen.Mapper/GetMapping',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_mapper_pb.GetMappingRequest,
    responseType: pulumi_codegen_mapper_pb.GetMappingResponse,
    requestSerialize: serialize_codegen_GetMappingRequest,
    requestDeserialize: deserialize_codegen_GetMappingRequest,
    responseSerialize: serialize_codegen_GetMappingResponse,
    responseDeserialize: deserialize_codegen_GetMappingResponse,
  },
};

exports.MapperClient = grpc.makeGenericClientConstructor(MapperService);
