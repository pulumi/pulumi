// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016, Pulumi Corporation.
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
var pulumi_workflow_pb = require('./workflow_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_pulumirpc_GenerateGraphRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GenerateGraphRequest)) {
    throw new Error('Expected argument of type pulumirpc.GenerateGraphRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateGraphRequest(buffer_arg) {
  return pulumi_workflow_pb.GenerateGraphRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateJobRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GenerateJobRequest)) {
    throw new Error('Expected argument of type pulumirpc.GenerateJobRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateJobRequest(buffer_arg) {
  return pulumi_workflow_pb.GenerateJobRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateNodeResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GenerateNodeResponse)) {
    throw new Error('Expected argument of type pulumirpc.GenerateNodeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateNodeResponse(buffer_arg) {
  return pulumi_workflow_pb.GenerateNodeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetGraphRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetGraphRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetGraphRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetGraphRequest(buffer_arg) {
  return pulumi_workflow_pb.GetGraphRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetGraphResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetGraphResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetGraphResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetGraphResponse(buffer_arg) {
  return pulumi_workflow_pb.GetGraphResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetGraphsRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetGraphsRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetGraphsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetGraphsRequest(buffer_arg) {
  return pulumi_workflow_pb.GetGraphsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetGraphsResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetGraphsResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetGraphsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetGraphsResponse(buffer_arg) {
  return pulumi_workflow_pb.GetGraphsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetJobRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetJobRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetJobRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetJobRequest(buffer_arg) {
  return pulumi_workflow_pb.GetJobRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetJobResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetJobResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetJobResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetJobResponse(buffer_arg) {
  return pulumi_workflow_pb.GetJobResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetJobsRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetJobsRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetJobsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetJobsRequest(buffer_arg) {
  return pulumi_workflow_pb.GetJobsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetJobsResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetJobsResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetJobsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetJobsResponse(buffer_arg) {
  return pulumi_workflow_pb.GetJobsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetPackageInfoRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetPackageInfoRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetPackageInfoRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetPackageInfoRequest(buffer_arg) {
  return pulumi_workflow_pb.GetPackageInfoRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetPackageInfoResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetPackageInfoResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetPackageInfoResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetPackageInfoResponse(buffer_arg) {
  return pulumi_workflow_pb.GetPackageInfoResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetStepResultRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetStepResultRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetStepResultRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetStepResultRequest(buffer_arg) {
  return pulumi_workflow_pb.GetStepResultRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetStepResultResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.GetStepResultResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetStepResultResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetStepResultResponse(buffer_arg) {
  return pulumi_workflow_pb.GetStepResultResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterGraphRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterGraphRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterGraphRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterGraphRequest(buffer_arg) {
  return pulumi_workflow_pb.RegisterGraphRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterJobRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterJobRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterJobRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterJobRequest(buffer_arg) {
  return pulumi_workflow_pb.RegisterJobRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterNodeResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterNodeResponse)) {
    throw new Error('Expected argument of type pulumirpc.RegisterNodeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterNodeResponse(buffer_arg) {
  return pulumi_workflow_pb.RegisterNodeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterSensorRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterSensorRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterSensorRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterSensorRequest(buffer_arg) {
  return pulumi_workflow_pb.RegisterSensorRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterStepRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterStepRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterStepRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterStepRequest(buffer_arg) {
  return pulumi_workflow_pb.RegisterStepRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RegisterTriggerRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RegisterTriggerRequest)) {
    throw new Error('Expected argument of type pulumirpc.RegisterTriggerRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RegisterTriggerRequest(buffer_arg) {
  return pulumi_workflow_pb.RegisterTriggerRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunFilterRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunFilterRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunFilterRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunFilterRequest(buffer_arg) {
  return pulumi_workflow_pb.RunFilterRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunFilterResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunFilterResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunFilterResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunFilterResponse(buffer_arg) {
  return pulumi_workflow_pb.RunFilterResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunOnErrorRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunOnErrorRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunOnErrorRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunOnErrorRequest(buffer_arg) {
  return pulumi_workflow_pb.RunOnErrorRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunOnErrorResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunOnErrorResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunOnErrorResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunOnErrorResponse(buffer_arg) {
  return pulumi_workflow_pb.RunOnErrorResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunSensorRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunSensorRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunSensorRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunSensorRequest(buffer_arg) {
  return pulumi_workflow_pb.RunSensorRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunSensorResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunSensorResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunSensorResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunSensorResponse(buffer_arg) {
  return pulumi_workflow_pb.RunSensorResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunStepRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunStepRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunStepRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunStepRequest(buffer_arg) {
  return pulumi_workflow_pb.RunStepRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunStepResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.RunStepResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunStepResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunStepResponse(buffer_arg) {
  return pulumi_workflow_pb.RunStepResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_WorkflowHandshakeRequest(arg) {
  if (!(arg instanceof pulumi_workflow_pb.WorkflowHandshakeRequest)) {
    throw new Error('Expected argument of type pulumirpc.WorkflowHandshakeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_WorkflowHandshakeRequest(buffer_arg) {
  return pulumi_workflow_pb.WorkflowHandshakeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_WorkflowHandshakeResponse(arg) {
  if (!(arg instanceof pulumi_workflow_pb.WorkflowHandshakeResponse)) {
    throw new Error('Expected argument of type pulumirpc.WorkflowHandshakeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_WorkflowHandshakeResponse(buffer_arg) {
  return pulumi_workflow_pb.WorkflowHandshakeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// WorkflowEvaluator is called by a scheduler/coordinator to incrementally discover
// schema metadata and execute/materialize specific workflow callable nodes.
var WorkflowEvaluatorService = exports.WorkflowEvaluatorService = {
  // `Handshake` is the first call made by the engine to a workflow evaluator. It is used to
// pass the engine's address to the evaluator so that it may establish its own connections
// back, and to establish protocol configuration that will be used to communicate between the
// two parties.
handshake: {
    path: '/pulumirpc.WorkflowEvaluator/Handshake',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.WorkflowHandshakeRequest,
    responseType: pulumi_workflow_pb.WorkflowHandshakeResponse,
    requestSerialize: serialize_pulumirpc_WorkflowHandshakeRequest,
    requestDeserialize: deserialize_pulumirpc_WorkflowHandshakeRequest,
    responseSerialize: serialize_pulumirpc_WorkflowHandshakeResponse,
    responseDeserialize: deserialize_pulumirpc_WorkflowHandshakeResponse,
  },
  // Returns high-level package metadata (name/version/display name/etc).
getPackageInfo: {
    path: '/pulumirpc.WorkflowEvaluator/GetPackageInfo',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetPackageInfoRequest,
    responseType: pulumi_workflow_pb.GetPackageInfoResponse,
    requestSerialize: serialize_pulumirpc_GetPackageInfoRequest,
    requestDeserialize: deserialize_pulumirpc_GetPackageInfoRequest,
    responseSerialize: serialize_pulumirpc_GetPackageInfoResponse,
    responseDeserialize: deserialize_pulumirpc_GetPackageInfoResponse,
  },
  // Returns the list of exported graph tokens.
getGraphs: {
    path: '/pulumirpc.WorkflowEvaluator/GetGraphs',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetGraphsRequest,
    responseType: pulumi_workflow_pb.GetGraphsResponse,
    requestSerialize: serialize_pulumirpc_GetGraphsRequest,
    requestDeserialize: deserialize_pulumirpc_GetGraphsRequest,
    responseSerialize: serialize_pulumirpc_GetGraphsResponse,
    responseDeserialize: deserialize_pulumirpc_GetGraphsResponse,
  },
  // Returns the schema for one exported graph.
getGraph: {
    path: '/pulumirpc.WorkflowEvaluator/GetGraph',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetGraphRequest,
    responseType: pulumi_workflow_pb.GetGraphResponse,
    requestSerialize: serialize_pulumirpc_GetGraphRequest,
    requestDeserialize: deserialize_pulumirpc_GetGraphRequest,
    responseSerialize: serialize_pulumirpc_GetGraphResponse,
    responseDeserialize: deserialize_pulumirpc_GetGraphResponse,
  },
  // Returns the list of exported job tokens.
getJobs: {
    path: '/pulumirpc.WorkflowEvaluator/GetJobs',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetJobsRequest,
    responseType: pulumi_workflow_pb.GetJobsResponse,
    requestSerialize: serialize_pulumirpc_GetJobsRequest,
    requestDeserialize: deserialize_pulumirpc_GetJobsRequest,
    responseSerialize: serialize_pulumirpc_GetJobsResponse,
    responseDeserialize: deserialize_pulumirpc_GetJobsResponse,
  },
  // Returns the schema for one exported job.
getJob: {
    path: '/pulumirpc.WorkflowEvaluator/GetJob',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetJobRequest,
    responseType: pulumi_workflow_pb.GetJobResponse,
    requestSerialize: serialize_pulumirpc_GetJobRequest,
    requestDeserialize: deserialize_pulumirpc_GetJobRequest,
    responseSerialize: serialize_pulumirpc_GetJobResponse,
    responseDeserialize: deserialize_pulumirpc_GetJobResponse,
  },
  // GenerateJob asks the evaluator to generate the job shape for a path.
generateJob: {
    path: '/pulumirpc.WorkflowEvaluator/GenerateJob',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GenerateJobRequest,
    responseType: pulumi_workflow_pb.GenerateNodeResponse,
    requestSerialize: serialize_pulumirpc_GenerateJobRequest,
    requestDeserialize: deserialize_pulumirpc_GenerateJobRequest,
    responseSerialize: serialize_pulumirpc_GenerateNodeResponse,
    responseDeserialize: deserialize_pulumirpc_GenerateNodeResponse,
  },
  // GenerateGraph asks the evaluator to generate the graph/subgraph shape for a path.
generateGraph: {
    path: '/pulumirpc.WorkflowEvaluator/GenerateGraph',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GenerateGraphRequest,
    responseType: pulumi_workflow_pb.GenerateNodeResponse,
    requestSerialize: serialize_pulumirpc_GenerateGraphRequest,
    requestDeserialize: deserialize_pulumirpc_GenerateGraphRequest,
    responseSerialize: serialize_pulumirpc_GenerateNodeResponse,
    responseDeserialize: deserialize_pulumirpc_GenerateNodeResponse,
  },
  // RunSensor executes a sensor poll function and returns fire/skip plus cursor data.
runSensor: {
    path: '/pulumirpc.WorkflowEvaluator/RunSensor',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RunSensorRequest,
    responseType: pulumi_workflow_pb.RunSensorResponse,
    requestSerialize: serialize_pulumirpc_RunSensorRequest,
    requestDeserialize: deserialize_pulumirpc_RunSensorRequest,
    responseSerialize: serialize_pulumirpc_RunSensorResponse,
    responseDeserialize: deserialize_pulumirpc_RunSensorResponse,
  },
  // RunStep executes a single step and returns a PropertyValue-compatible result.
runStep: {
    path: '/pulumirpc.WorkflowEvaluator/RunStep',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RunStepRequest,
    responseType: pulumi_workflow_pb.RunStepResponse,
    requestSerialize: serialize_pulumirpc_RunStepRequest,
    requestDeserialize: deserialize_pulumirpc_RunStepRequest,
    responseSerialize: serialize_pulumirpc_RunStepResponse,
    responseDeserialize: deserialize_pulumirpc_RunStepResponse,
  },
  // RunFilter executes a trigger filter and returns pass/fail.
runFilter: {
    path: '/pulumirpc.WorkflowEvaluator/RunFilter',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RunFilterRequest,
    responseType: pulumi_workflow_pb.RunFilterResponse,
    requestSerialize: serialize_pulumirpc_RunFilterRequest,
    requestDeserialize: deserialize_pulumirpc_RunFilterRequest,
    responseSerialize: serialize_pulumirpc_RunFilterResponse,
    responseDeserialize: deserialize_pulumirpc_RunFilterResponse,
  },
  // RunOnError executes a node's on-error callback and returns retry behavior.
runOnError: {
    path: '/pulumirpc.WorkflowEvaluator/RunOnError',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RunOnErrorRequest,
    responseType: pulumi_workflow_pb.RunOnErrorResponse,
    requestSerialize: serialize_pulumirpc_RunOnErrorRequest,
    requestDeserialize: deserialize_pulumirpc_RunOnErrorRequest,
    responseSerialize: serialize_pulumirpc_RunOnErrorResponse,
    responseDeserialize: deserialize_pulumirpc_RunOnErrorResponse,
  },
};

exports.WorkflowEvaluatorClient = grpc.makeGenericClientConstructor(WorkflowEvaluatorService, 'WorkflowEvaluator');
// GraphMonitor is called while evaluating a concrete graph execution/generation.
// It records the graph shape for that evaluation and resolves prior node outputs.
var GraphMonitorService = exports.GraphMonitorService = {
  registerTrigger: {
    path: '/pulumirpc.GraphMonitor/RegisterTrigger',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RegisterTriggerRequest,
    responseType: pulumi_workflow_pb.RegisterNodeResponse,
    requestSerialize: serialize_pulumirpc_RegisterTriggerRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterTriggerRequest,
    responseSerialize: serialize_pulumirpc_RegisterNodeResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterNodeResponse,
  },
  registerSensor: {
    path: '/pulumirpc.GraphMonitor/RegisterSensor',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RegisterSensorRequest,
    responseType: pulumi_workflow_pb.RegisterNodeResponse,
    requestSerialize: serialize_pulumirpc_RegisterSensorRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterSensorRequest,
    responseSerialize: serialize_pulumirpc_RegisterNodeResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterNodeResponse,
  },
  registerJob: {
    path: '/pulumirpc.GraphMonitor/RegisterJob',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RegisterJobRequest,
    responseType: pulumi_workflow_pb.RegisterNodeResponse,
    requestSerialize: serialize_pulumirpc_RegisterJobRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterJobRequest,
    responseSerialize: serialize_pulumirpc_RegisterNodeResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterNodeResponse,
  },
  registerGraph: {
    path: '/pulumirpc.GraphMonitor/RegisterGraph',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RegisterGraphRequest,
    responseType: pulumi_workflow_pb.RegisterNodeResponse,
    requestSerialize: serialize_pulumirpc_RegisterGraphRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterGraphRequest,
    responseSerialize: serialize_pulumirpc_RegisterNodeResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterNodeResponse,
  },
  registerStep: {
    path: '/pulumirpc.GraphMonitor/RegisterStep',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.RegisterStepRequest,
    responseType: pulumi_workflow_pb.RegisterNodeResponse,
    requestSerialize: serialize_pulumirpc_RegisterStepRequest,
    requestDeserialize: deserialize_pulumirpc_RegisterStepRequest,
    responseSerialize: serialize_pulumirpc_RegisterNodeResponse,
    responseDeserialize: deserialize_pulumirpc_RegisterNodeResponse,
  },
  // GetStepResult asks for a previously completed step output.
getStepResult: {
    path: '/pulumirpc.GraphMonitor/GetStepResult',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_workflow_pb.GetStepResultRequest,
    responseType: pulumi_workflow_pb.GetStepResultResponse,
    requestSerialize: serialize_pulumirpc_GetStepResultRequest,
    requestDeserialize: deserialize_pulumirpc_GetStepResultRequest,
    responseSerialize: serialize_pulumirpc_GetStepResultResponse,
    responseDeserialize: deserialize_pulumirpc_GetStepResultResponse,
  },
};

exports.GraphMonitorClient = grpc.makeGenericClientConstructor(GraphMonitorService, 'GraphMonitor');
