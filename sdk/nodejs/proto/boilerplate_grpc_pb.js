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
var pulumi_boilerplate_pb = require('./boilerplate_pb.js');
var pulumi_codegen_hcl_pb = require('./codegen/hcl_pb.js');
var pulumi_plugin_pb = require('./plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_pulumirpc_CreatePackageRequest(arg) {
  if (!(arg instanceof pulumi_boilerplate_pb.CreatePackageRequest)) {
    throw new Error('Expected argument of type pulumirpc.CreatePackageRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CreatePackageRequest(buffer_arg) {
  return pulumi_boilerplate_pb.CreatePackageRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CreatePackageResponse(arg) {
  if (!(arg instanceof pulumi_boilerplate_pb.CreatePackageResponse)) {
    throw new Error('Expected argument of type pulumirpc.CreatePackageResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CreatePackageResponse(buffer_arg) {
  return pulumi_boilerplate_pb.CreatePackageResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Boilerplate is a service for creating Pulumi packages - providers.
var BoilerplateService = exports.BoilerplateService = {
  // CreatePackage creates a new Pulumi package - provider.
createPackage: {
    path: '/pulumirpc.Boilerplate/CreatePackage',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_boilerplate_pb.CreatePackageRequest,
    responseType: pulumi_boilerplate_pb.CreatePackageResponse,
    requestSerialize: serialize_pulumirpc_CreatePackageRequest,
    requestDeserialize: deserialize_pulumirpc_CreatePackageRequest,
    responseSerialize: serialize_pulumirpc_CreatePackageResponse,
    responseDeserialize: deserialize_pulumirpc_CreatePackageResponse,
  },
};

exports.BoilerplateClient = grpc.makeGenericClientConstructor(BoilerplateService);
