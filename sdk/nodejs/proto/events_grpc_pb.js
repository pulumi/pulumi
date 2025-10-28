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
var pulumi_events_pb = require('./events_pb.js');
var pulumi_codegen_hcl_pb = require('./codegen/hcl_pb.js');
var pulumi_plugin_pb = require('./plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_EventRequest(arg) {
  if (!(arg instanceof pulumi_events_pb.EventRequest)) {
    throw new Error('Expected argument of type pulumirpc.EventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_EventRequest(buffer_arg) {
  return pulumi_events_pb.EventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


var EventsService = exports.EventsService = {
  event: {
    path: '/pulumirpc.Events/Event',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_events_pb.EventRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_EventRequest,
    requestDeserialize: deserialize_pulumirpc_EventRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  done: {
    path: '/pulumirpc.Events/Done',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.EventsClient = grpc.makeGenericClientConstructor(EventsService, 'Events');
