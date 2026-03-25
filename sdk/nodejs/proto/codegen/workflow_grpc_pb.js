// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2026, Pulumi Corporation.
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
var pulumi_codegen_workflow_pb = require('../codegen/workflow_pb.js');

function serialize_codegen_GetGraphResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetGraphResponse)) {
    throw new Error('Expected argument of type codegen.GetGraphResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetGraphResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetGraphResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetGraphsResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetGraphsResponse)) {
    throw new Error('Expected argument of type codegen.GetGraphsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetGraphsResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetGraphsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetJobResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetJobResponse)) {
    throw new Error('Expected argument of type codegen.GetJobResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetJobResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetJobResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetJobsResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetJobsResponse)) {
    throw new Error('Expected argument of type codegen.GetJobsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetJobsResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetJobsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetPackageInfoResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetPackageInfoResponse)) {
    throw new Error('Expected argument of type codegen.GetPackageInfoResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetPackageInfoResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetPackageInfoResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetTriggerResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetTriggerResponse)) {
    throw new Error('Expected argument of type codegen.GetTriggerResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetTriggerResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetTriggerResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetTriggersResponse(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetTriggersResponse)) {
    throw new Error('Expected argument of type codegen.GetTriggersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetTriggersResponse(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetTriggersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowGraphRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowGraphRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowGraphRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowGraphRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowGraphRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowGraphsRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowGraphsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowGraphsRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowJobRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowJobRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowJobRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowJobRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowJobRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowJobsRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowJobsRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowJobsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowJobsRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowJobsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowPackageInfoRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowPackageInfoRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowPackageInfoRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowTriggerRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowTriggerRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowTriggerRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_codegen_GetWorkflowTriggersRequest(arg) {
  if (!(arg instanceof pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest)) {
    throw new Error('Expected argument of type codegen.GetWorkflowTriggersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_codegen_GetWorkflowTriggersRequest(buffer_arg) {
  return pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


// WorkflowLoader provides schema-fetching APIs for workflow package SDK generation.
var WorkflowLoaderService = exports.WorkflowLoaderService = {
  getPackageInfo: {
    path: '/codegen.WorkflowLoader/GetPackageInfo',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowPackageInfoRequest,
    responseType: pulumi_codegen_workflow_pb.GetPackageInfoResponse,
    requestSerialize: serialize_codegen_GetWorkflowPackageInfoRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowPackageInfoRequest,
    responseSerialize: serialize_codegen_GetPackageInfoResponse,
    responseDeserialize: deserialize_codegen_GetPackageInfoResponse,
  },
  getGraphs: {
    path: '/codegen.WorkflowLoader/GetGraphs',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowGraphsRequest,
    responseType: pulumi_codegen_workflow_pb.GetGraphsResponse,
    requestSerialize: serialize_codegen_GetWorkflowGraphsRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowGraphsRequest,
    responseSerialize: serialize_codegen_GetGraphsResponse,
    responseDeserialize: deserialize_codegen_GetGraphsResponse,
  },
  getGraph: {
    path: '/codegen.WorkflowLoader/GetGraph',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowGraphRequest,
    responseType: pulumi_codegen_workflow_pb.GetGraphResponse,
    requestSerialize: serialize_codegen_GetWorkflowGraphRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowGraphRequest,
    responseSerialize: serialize_codegen_GetGraphResponse,
    responseDeserialize: deserialize_codegen_GetGraphResponse,
  },
  getTriggers: {
    path: '/codegen.WorkflowLoader/GetTriggers',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowTriggersRequest,
    responseType: pulumi_codegen_workflow_pb.GetTriggersResponse,
    requestSerialize: serialize_codegen_GetWorkflowTriggersRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowTriggersRequest,
    responseSerialize: serialize_codegen_GetTriggersResponse,
    responseDeserialize: deserialize_codegen_GetTriggersResponse,
  },
  getTrigger: {
    path: '/codegen.WorkflowLoader/GetTrigger',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowTriggerRequest,
    responseType: pulumi_codegen_workflow_pb.GetTriggerResponse,
    requestSerialize: serialize_codegen_GetWorkflowTriggerRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowTriggerRequest,
    responseSerialize: serialize_codegen_GetTriggerResponse,
    responseDeserialize: deserialize_codegen_GetTriggerResponse,
  },
  getJobs: {
    path: '/codegen.WorkflowLoader/GetJobs',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowJobsRequest,
    responseType: pulumi_codegen_workflow_pb.GetJobsResponse,
    requestSerialize: serialize_codegen_GetWorkflowJobsRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowJobsRequest,
    responseSerialize: serialize_codegen_GetJobsResponse,
    responseDeserialize: deserialize_codegen_GetJobsResponse,
  },
  getJob: {
    path: '/codegen.WorkflowLoader/GetJob',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_codegen_workflow_pb.GetWorkflowJobRequest,
    responseType: pulumi_codegen_workflow_pb.GetJobResponse,
    requestSerialize: serialize_codegen_GetWorkflowJobRequest,
    requestDeserialize: deserialize_codegen_GetWorkflowJobRequest,
    responseSerialize: serialize_codegen_GetJobResponse,
    responseDeserialize: deserialize_codegen_GetJobResponse,
  },
};

exports.WorkflowLoaderClient = grpc.makeGenericClientConstructor(WorkflowLoaderService, 'WorkflowLoader');
