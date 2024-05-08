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
var pulumi_converter_pb = require('./converter_pb.js');
var pulumi_codegen_hcl_pb = require('./codegen/hcl_pb.js');

function serialize_pulumirpc_ConvertProgramRequest(arg) {
  if (!(arg instanceof pulumi_converter_pb.ConvertProgramRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConvertProgramRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConvertProgramRequest(buffer_arg) {
  return pulumi_converter_pb.ConvertProgramRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConvertProgramResponse(arg) {
  if (!(arg instanceof pulumi_converter_pb.ConvertProgramResponse)) {
    throw new Error('Expected argument of type pulumirpc.ConvertProgramResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConvertProgramResponse(buffer_arg) {
  return pulumi_converter_pb.ConvertProgramResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConvertStateRequest(arg) {
  if (!(arg instanceof pulumi_converter_pb.ConvertStateRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConvertStateRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConvertStateRequest(buffer_arg) {
  return pulumi_converter_pb.ConvertStateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConvertStateResponse(arg) {
  if (!(arg instanceof pulumi_converter_pb.ConvertStateResponse)) {
    throw new Error('Expected argument of type pulumirpc.ConvertStateResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConvertStateResponse(buffer_arg) {
  return pulumi_converter_pb.ConvertStateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// Converter is a service for converting between other ecosystems and Pulumi.
// This is currently unstable and experimental.
var ConverterService = exports.ConverterService = {
  // ConvertState converts state from the target ecosystem into a form that can be imported into Pulumi.
convertState: {
    path: '/pulumirpc.Converter/ConvertState',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_converter_pb.ConvertStateRequest,
    responseType: pulumi_converter_pb.ConvertStateResponse,
    requestSerialize: serialize_pulumirpc_ConvertStateRequest,
    requestDeserialize: deserialize_pulumirpc_ConvertStateRequest,
    responseSerialize: serialize_pulumirpc_ConvertStateResponse,
    responseDeserialize: deserialize_pulumirpc_ConvertStateResponse,
  },
  // ConvertProgram converts a program from the target ecosystem into a form that can be used with Pulumi.
convertProgram: {
    path: '/pulumirpc.Converter/ConvertProgram',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_converter_pb.ConvertProgramRequest,
    responseType: pulumi_converter_pb.ConvertProgramResponse,
    requestSerialize: serialize_pulumirpc_ConvertProgramRequest,
    requestDeserialize: deserialize_pulumirpc_ConvertProgramRequest,
    responseSerialize: serialize_pulumirpc_ConvertProgramResponse,
    responseDeserialize: deserialize_pulumirpc_ConvertProgramResponse,
  },
};

exports.ConverterClient = grpc.makeGenericClientConstructor(ConverterService);
