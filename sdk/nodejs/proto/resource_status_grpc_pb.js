// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2025, Pulumi Corporation.
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
var pulumi_resource_status_pb = require('./resource_status_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
var pulumi_provider_pb = require('./provider_pb.js');

function serialize_pulumirpc_PublishViewStepsRequest(arg) {
  if (!(arg instanceof pulumi_resource_status_pb.PublishViewStepsRequest)) {
    throw new Error('Expected argument of type pulumirpc.PublishViewStepsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PublishViewStepsRequest(buffer_arg) {
  return pulumi_resource_status_pb.PublishViewStepsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PublishViewStepsResponse(arg) {
  if (!(arg instanceof pulumi_resource_status_pb.PublishViewStepsResponse)) {
    throw new Error('Expected argument of type pulumirpc.PublishViewStepsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PublishViewStepsResponse(buffer_arg) {
  return pulumi_resource_status_pb.PublishViewStepsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// ResourceStatus is an interface that can be called from a resource provider to update status about a resource.
var ResourceStatusService = exports.ResourceStatusService = {
  // `PublishViewSteps` is used to publish a series of steps for a view resource.
// Views can be materialized via create and update steps, and more complex
// changes, such as replacements, can be modeled as a series of steps.
// The engine does not actually apply these steps, but rather flows them through
// the engine such that the view resources are written to state and the view
// resources are displayed in the UI.
publishViewSteps: {
    path: '/pulumirpc.ResourceStatus/PublishViewSteps',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_resource_status_pb.PublishViewStepsRequest,
    responseType: pulumi_resource_status_pb.PublishViewStepsResponse,
    requestSerialize: serialize_pulumirpc_PublishViewStepsRequest,
    requestDeserialize: deserialize_pulumirpc_PublishViewStepsRequest,
    responseSerialize: serialize_pulumirpc_PublishViewStepsResponse,
    responseDeserialize: deserialize_pulumirpc_PublishViewStepsResponse,
  },
};

exports.ResourceStatusClient = grpc.makeGenericClientConstructor(ResourceStatusService, 'ResourceStatus');
