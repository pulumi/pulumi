// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2022, Pulumi Corporation.
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
var grpc = require('grpc');
var pulumi_secrets_secrets_pb = require('../../pulumi/secrets/secrets_pb.js');
var pulumi_plugin_pb = require('../../pulumi/plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_pulumirpc_secrets_ConfigureRequest(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.ConfigureRequest)) {
    throw new Error('Expected argument of type pulumirpc.secrets.ConfigureRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_ConfigureRequest(buffer_arg) {
  return pulumi_secrets_secrets_pb.ConfigureRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_ConfigureResponse(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.ConfigureResponse)) {
    throw new Error('Expected argument of type pulumirpc.secrets.ConfigureResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_ConfigureResponse(buffer_arg) {
  return pulumi_secrets_secrets_pb.ConfigureResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_DecryptRequest(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.DecryptRequest)) {
    throw new Error('Expected argument of type pulumirpc.secrets.DecryptRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_DecryptRequest(buffer_arg) {
  return pulumi_secrets_secrets_pb.DecryptRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_DecryptResponse(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.DecryptResponse)) {
    throw new Error('Expected argument of type pulumirpc.secrets.DecryptResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_DecryptResponse(buffer_arg) {
  return pulumi_secrets_secrets_pb.DecryptResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_EncryptRequest(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.EncryptRequest)) {
    throw new Error('Expected argument of type pulumirpc.secrets.EncryptRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_EncryptRequest(buffer_arg) {
  return pulumi_secrets_secrets_pb.EncryptRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_EncryptResponse(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.EncryptResponse)) {
    throw new Error('Expected argument of type pulumirpc.secrets.EncryptResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_EncryptResponse(buffer_arg) {
  return pulumi_secrets_secrets_pb.EncryptResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_InitializeRequest(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.InitializeRequest)) {
    throw new Error('Expected argument of type pulumirpc.secrets.InitializeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_InitializeRequest(buffer_arg) {
  return pulumi_secrets_secrets_pb.InitializeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_secrets_InitializeResponse(arg) {
  if (!(arg instanceof pulumi_secrets_secrets_pb.InitializeResponse)) {
    throw new Error('Expected argument of type pulumirpc.secrets.InitializeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_secrets_InitializeResponse(buffer_arg) {
  return pulumi_secrets_secrets_pb.InitializeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// SecretsProvider is a service that understands how to encrypt and decrypt secrets.
var SecretsProviderService = exports.SecretsProviderService = {
  // Encrypt encrypts plaintext to ciphertext.
encrypt: {
    path: '/pulumirpc.secrets.SecretsProvider/Encrypt',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_secrets_secrets_pb.EncryptRequest,
    responseType: pulumi_secrets_secrets_pb.EncryptResponse,
    requestSerialize: serialize_pulumirpc_secrets_EncryptRequest,
    requestDeserialize: deserialize_pulumirpc_secrets_EncryptRequest,
    responseSerialize: serialize_pulumirpc_secrets_EncryptResponse,
    responseDeserialize: deserialize_pulumirpc_secrets_EncryptResponse,
  },
  // Decrypt decrypts ciphertext to plaintext.
decrypt: {
    path: '/pulumirpc.secrets.SecretsProvider/Decrypt',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_secrets_secrets_pb.DecryptRequest,
    responseType: pulumi_secrets_secrets_pb.DecryptResponse,
    requestSerialize: serialize_pulumirpc_secrets_DecryptRequest,
    requestDeserialize: deserialize_pulumirpc_secrets_DecryptRequest,
    responseSerialize: serialize_pulumirpc_secrets_DecryptResponse,
    responseDeserialize: deserialize_pulumirpc_secrets_DecryptResponse,
  },
  // Configure setups a secrets provider from existing state.
configure: {
    path: '/pulumirpc.secrets.SecretsProvider/Configure',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_secrets_secrets_pb.ConfigureRequest,
    responseType: pulumi_secrets_secrets_pb.ConfigureResponse,
    requestSerialize: serialize_pulumirpc_secrets_ConfigureRequest,
    requestDeserialize: deserialize_pulumirpc_secrets_ConfigureRequest,
    responseSerialize: serialize_pulumirpc_secrets_ConfigureResponse,
    responseDeserialize: deserialize_pulumirpc_secrets_ConfigureResponse,
  },
  // Initialize setups a new secrets provider.
initialize: {
    path: '/pulumirpc.secrets.SecretsProvider/Initialize',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_secrets_secrets_pb.InitializeRequest,
    responseType: pulumi_secrets_secrets_pb.InitializeResponse,
    requestSerialize: serialize_pulumirpc_secrets_InitializeRequest,
    requestDeserialize: deserialize_pulumirpc_secrets_InitializeRequest,
    responseSerialize: serialize_pulumirpc_secrets_InitializeResponse,
    responseDeserialize: deserialize_pulumirpc_secrets_InitializeResponse,
  },
};

exports.SecretsProviderClient = grpc.makeGenericClientConstructor(SecretsProviderService);
