// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2018, Pulumi Corporation.
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
var pulumi_provider_pb = require('./provider_pb.js');
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

function serialize_pulumirpc_CallRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.CallRequest)) {
    throw new Error('Expected argument of type pulumirpc.CallRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CallRequest(buffer_arg) {
  return pulumi_provider_pb.CallRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CallResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.CallResponse)) {
    throw new Error('Expected argument of type pulumirpc.CallResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CallResponse(buffer_arg) {
  return pulumi_provider_pb.CallResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CheckRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.CheckRequest)) {
    throw new Error('Expected argument of type pulumirpc.CheckRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CheckRequest(buffer_arg) {
  return pulumi_provider_pb.CheckRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CheckResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.CheckResponse)) {
    throw new Error('Expected argument of type pulumirpc.CheckResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CheckResponse(buffer_arg) {
  return pulumi_provider_pb.CheckResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConfigureRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.ConfigureRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConfigureRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConfigureRequest(buffer_arg) {
  return pulumi_provider_pb.ConfigureRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConfigureResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.ConfigureResponse)) {
    throw new Error('Expected argument of type pulumirpc.ConfigureResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConfigureResponse(buffer_arg) {
  return pulumi_provider_pb.ConfigureResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConstructRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.ConstructRequest)) {
    throw new Error('Expected argument of type pulumirpc.ConstructRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConstructRequest(buffer_arg) {
  return pulumi_provider_pb.ConstructRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ConstructResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.ConstructResponse)) {
    throw new Error('Expected argument of type pulumirpc.ConstructResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ConstructResponse(buffer_arg) {
  return pulumi_provider_pb.ConstructResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CreateRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.CreateRequest)) {
    throw new Error('Expected argument of type pulumirpc.CreateRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CreateRequest(buffer_arg) {
  return pulumi_provider_pb.CreateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_CreateResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.CreateResponse)) {
    throw new Error('Expected argument of type pulumirpc.CreateResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_CreateResponse(buffer_arg) {
  return pulumi_provider_pb.CreateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_DeleteRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.DeleteRequest)) {
    throw new Error('Expected argument of type pulumirpc.DeleteRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_DeleteRequest(buffer_arg) {
  return pulumi_provider_pb.DeleteRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_DiffRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.DiffRequest)) {
    throw new Error('Expected argument of type pulumirpc.DiffRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_DiffRequest(buffer_arg) {
  return pulumi_provider_pb.DiffRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_DiffResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.DiffResponse)) {
    throw new Error('Expected argument of type pulumirpc.DiffResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_DiffResponse(buffer_arg) {
  return pulumi_provider_pb.DiffResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetMappingRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.GetMappingRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetMappingRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetMappingRequest(buffer_arg) {
  return pulumi_provider_pb.GetMappingRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetMappingResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.GetMappingResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetMappingResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetMappingResponse(buffer_arg) {
  return pulumi_provider_pb.GetMappingResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetSchemaRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.GetSchemaRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetSchemaRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetSchemaRequest(buffer_arg) {
  return pulumi_provider_pb.GetSchemaRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetSchemaResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.GetSchemaResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetSchemaResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetSchemaResponse(buffer_arg) {
  return pulumi_provider_pb.GetSchemaResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.InvokeRequest)) {
    throw new Error('Expected argument of type pulumirpc.InvokeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeRequest(buffer_arg) {
  return pulumi_provider_pb.InvokeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InvokeResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.InvokeResponse)) {
    throw new Error('Expected argument of type pulumirpc.InvokeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InvokeResponse(buffer_arg) {
  return pulumi_provider_pb.InvokeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PluginAttach(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PluginAttach)) {
    throw new Error('Expected argument of type pulumirpc.PluginAttach');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PluginAttach(buffer_arg) {
  return pulumi_plugin_pb.PluginAttach.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PluginInfo(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PluginInfo)) {
    throw new Error('Expected argument of type pulumirpc.PluginInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PluginInfo(buffer_arg) {
  return pulumi_plugin_pb.PluginInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ReadRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.ReadRequest)) {
    throw new Error('Expected argument of type pulumirpc.ReadRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ReadRequest(buffer_arg) {
  return pulumi_provider_pb.ReadRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ReadResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.ReadResponse)) {
    throw new Error('Expected argument of type pulumirpc.ReadResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ReadResponse(buffer_arg) {
  return pulumi_provider_pb.ReadResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_UpdateRequest(arg) {
  if (!(arg instanceof pulumi_provider_pb.UpdateRequest)) {
    throw new Error('Expected argument of type pulumirpc.UpdateRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_UpdateRequest(buffer_arg) {
  return pulumi_provider_pb.UpdateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_UpdateResponse(arg) {
  if (!(arg instanceof pulumi_provider_pb.UpdateResponse)) {
    throw new Error('Expected argument of type pulumirpc.UpdateResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_UpdateResponse(buffer_arg) {
  return pulumi_provider_pb.UpdateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// ResourceProvider is a service that understands how to create, read, update, or delete resources for types defined
// within a single package.  It is driven by the overall planning engine in response to resource diffs.
var ResourceProviderService = exports.ResourceProviderService = {
  // GetSchema fetches the schema for this resource provider.
getSchema: {
    path: '/pulumirpc.ResourceProvider/GetSchema',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.GetSchemaRequest,
    responseType: pulumi_provider_pb.GetSchemaResponse,
    requestSerialize: serialize_pulumirpc_GetSchemaRequest,
    requestDeserialize: deserialize_pulumirpc_GetSchemaRequest,
    responseSerialize: serialize_pulumirpc_GetSchemaResponse,
    responseDeserialize: deserialize_pulumirpc_GetSchemaResponse,
  },
  // CheckConfig validates the configuration for this resource provider.
checkConfig: {
    path: '/pulumirpc.ResourceProvider/CheckConfig',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.CheckRequest,
    responseType: pulumi_provider_pb.CheckResponse,
    requestSerialize: serialize_pulumirpc_CheckRequest,
    requestDeserialize: deserialize_pulumirpc_CheckRequest,
    responseSerialize: serialize_pulumirpc_CheckResponse,
    responseDeserialize: deserialize_pulumirpc_CheckResponse,
  },
  // DiffConfig checks the impact a hypothetical change to this provider's configuration will have on the provider.
diffConfig: {
    path: '/pulumirpc.ResourceProvider/DiffConfig',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.DiffRequest,
    responseType: pulumi_provider_pb.DiffResponse,
    requestSerialize: serialize_pulumirpc_DiffRequest,
    requestDeserialize: deserialize_pulumirpc_DiffRequest,
    responseSerialize: serialize_pulumirpc_DiffResponse,
    responseDeserialize: deserialize_pulumirpc_DiffResponse,
  },
  // Configure configures the resource provider with "globals" that control its behavior.
configure: {
    path: '/pulumirpc.ResourceProvider/Configure',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.ConfigureRequest,
    responseType: pulumi_provider_pb.ConfigureResponse,
    requestSerialize: serialize_pulumirpc_ConfigureRequest,
    requestDeserialize: deserialize_pulumirpc_ConfigureRequest,
    responseSerialize: serialize_pulumirpc_ConfigureResponse,
    responseDeserialize: deserialize_pulumirpc_ConfigureResponse,
  },
  // Invoke dynamically executes a built-in function in the provider.
invoke: {
    path: '/pulumirpc.ResourceProvider/Invoke',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.InvokeRequest,
    responseType: pulumi_provider_pb.InvokeResponse,
    requestSerialize: serialize_pulumirpc_InvokeRequest,
    requestDeserialize: deserialize_pulumirpc_InvokeRequest,
    responseSerialize: serialize_pulumirpc_InvokeResponse,
    responseDeserialize: deserialize_pulumirpc_InvokeResponse,
  },
  // StreamInvoke dynamically executes a built-in function in the provider, which returns a stream
// of responses.
streamInvoke: {
    path: '/pulumirpc.ResourceProvider/StreamInvoke',
    requestStream: false,
    responseStream: true,
    requestType: pulumi_provider_pb.InvokeRequest,
    responseType: pulumi_provider_pb.InvokeResponse,
    requestSerialize: serialize_pulumirpc_InvokeRequest,
    requestDeserialize: deserialize_pulumirpc_InvokeRequest,
    responseSerialize: serialize_pulumirpc_InvokeResponse,
    responseDeserialize: deserialize_pulumirpc_InvokeResponse,
  },
  // Call dynamically executes a method in the provider associated with a component resource.
call: {
    path: '/pulumirpc.ResourceProvider/Call',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.CallRequest,
    responseType: pulumi_provider_pb.CallResponse,
    requestSerialize: serialize_pulumirpc_CallRequest,
    requestDeserialize: deserialize_pulumirpc_CallRequest,
    responseSerialize: serialize_pulumirpc_CallResponse,
    responseDeserialize: deserialize_pulumirpc_CallResponse,
  },
  // Check validates that the given property bag is valid for a resource of the given type and returns the inputs
// that should be passed to successive calls to Diff, Create, or Update for this resource. As a rule, the provider
// inputs returned by a call to Check should preserve the original representation of the properties as present in
// the program inputs. Though this rule is not required for correctness, violations thereof can negatively impact
// the end-user experience, as the provider inputs are using for detecting and rendering diffs.
check: {
    path: '/pulumirpc.ResourceProvider/Check',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.CheckRequest,
    responseType: pulumi_provider_pb.CheckResponse,
    requestSerialize: serialize_pulumirpc_CheckRequest,
    requestDeserialize: deserialize_pulumirpc_CheckRequest,
    responseSerialize: serialize_pulumirpc_CheckResponse,
    responseDeserialize: deserialize_pulumirpc_CheckResponse,
  },
  // Diff checks what impacts a hypothetical update will have on the resource's properties.
diff: {
    path: '/pulumirpc.ResourceProvider/Diff',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.DiffRequest,
    responseType: pulumi_provider_pb.DiffResponse,
    requestSerialize: serialize_pulumirpc_DiffRequest,
    requestDeserialize: deserialize_pulumirpc_DiffRequest,
    responseSerialize: serialize_pulumirpc_DiffResponse,
    responseDeserialize: deserialize_pulumirpc_DiffResponse,
  },
  // Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transactional").
create: {
    path: '/pulumirpc.ResourceProvider/Create',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.CreateRequest,
    responseType: pulumi_provider_pb.CreateResponse,
    requestSerialize: serialize_pulumirpc_CreateRequest,
    requestDeserialize: deserialize_pulumirpc_CreateRequest,
    responseSerialize: serialize_pulumirpc_CreateResponse,
    responseDeserialize: deserialize_pulumirpc_CreateResponse,
  },
  // Read the current live state associated with a resource.  Enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource ID, but may also include some properties.
read: {
    path: '/pulumirpc.ResourceProvider/Read',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.ReadRequest,
    responseType: pulumi_provider_pb.ReadResponse,
    requestSerialize: serialize_pulumirpc_ReadRequest,
    requestDeserialize: deserialize_pulumirpc_ReadRequest,
    responseSerialize: serialize_pulumirpc_ReadResponse,
    responseDeserialize: deserialize_pulumirpc_ReadResponse,
  },
  // Update updates an existing resource with new values.
update: {
    path: '/pulumirpc.ResourceProvider/Update',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.UpdateRequest,
    responseType: pulumi_provider_pb.UpdateResponse,
    requestSerialize: serialize_pulumirpc_UpdateRequest,
    requestDeserialize: deserialize_pulumirpc_UpdateRequest,
    responseSerialize: serialize_pulumirpc_UpdateResponse,
    responseDeserialize: deserialize_pulumirpc_UpdateResponse,
  },
  // Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
delete: {
    path: '/pulumirpc.ResourceProvider/Delete',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.DeleteRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_DeleteRequest,
    requestDeserialize: deserialize_pulumirpc_DeleteRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // Construct creates a new instance of the provided component resource and returns its state.
construct: {
    path: '/pulumirpc.ResourceProvider/Construct',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.ConstructRequest,
    responseType: pulumi_provider_pb.ConstructResponse,
    requestSerialize: serialize_pulumirpc_ConstructRequest,
    requestDeserialize: deserialize_pulumirpc_ConstructRequest,
    responseSerialize: serialize_pulumirpc_ConstructResponse,
    responseDeserialize: deserialize_pulumirpc_ConstructResponse,
  },
  // Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either return a
// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
// to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
cancel: {
    path: '/pulumirpc.ResourceProvider/Cancel',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // GetPluginInfo returns generic information about this plugin, like its version.
getPluginInfo: {
    path: '/pulumirpc.ResourceProvider/GetPluginInfo',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_plugin_pb.PluginInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_PluginInfo,
    responseDeserialize: deserialize_pulumirpc_PluginInfo,
  },
  // Attach sends the engine address to an already running plugin.
attach: {
    path: '/pulumirpc.ResourceProvider/Attach',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_plugin_pb.PluginAttach,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_pulumirpc_PluginAttach,
    requestDeserialize: deserialize_pulumirpc_PluginAttach,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // GetMapping fetches the mapping for this resource provider, if any. A provider should return an empty
// response (not an error) if it doesn't have a mapping for the given key.
getMapping: {
    path: '/pulumirpc.ResourceProvider/GetMapping',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_provider_pb.GetMappingRequest,
    responseType: pulumi_provider_pb.GetMappingResponse,
    requestSerialize: serialize_pulumirpc_GetMappingRequest,
    requestDeserialize: deserialize_pulumirpc_GetMappingRequest,
    responseSerialize: serialize_pulumirpc_GetMappingResponse,
    responseDeserialize: deserialize_pulumirpc_GetMappingResponse,
  },
};

exports.ResourceProviderClient = grpc.makeGenericClientConstructor(ResourceProviderService);
