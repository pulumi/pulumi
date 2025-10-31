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
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');

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


// Events is a service for receiving engine events over gRPC.
// This service allows the Pulumi CLI to send engine events to clients
// (such as the Automation API) over a gRPC stream instead of writing them to
// a file on the filesystem and reading them from there.
var EventsService = exports.EventsService = {
  // StreamEvents allows the client to stream multiple engine events to the server.
// The client sends multiple EventRequest messages over the stream, and the server
// processes them as they arrive. When the client is done sending events, it closes
// the stream.
streamEvents: {
    path: '/pulumirpc.Events/StreamEvents',
    requestStream: true,
    responseStream: false,
    requestType: pulumi_events_pb.EventRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_EventRequest,
    requestDeserialize: deserialize_pulumirpc_EventRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.EventsClient = grpc.makeGenericClientConstructor(EventsService, 'Events');
