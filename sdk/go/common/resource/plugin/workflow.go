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

package plugin

import (
	"context"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Workflow represents a workflow evaluator plugin process.
type Workflow interface {
	Close() error
	Handshake(context.Context, *pulumirpc.WorkflowHandshakeRequest) (*pulumirpc.WorkflowHandshakeResponse, error)
	GetPackageInfo(context.Context, *pulumirpc.GetPackageInfoRequest) (*pulumirpc.GetPackageInfoResponse, error)
	GetGraphs(context.Context, *pulumirpc.GetGraphsRequest) (*pulumirpc.GetGraphsResponse, error)
	GetGraph(context.Context, *pulumirpc.GetGraphRequest) (*pulumirpc.GetGraphResponse, error)
	GetTriggers(context.Context, *pulumirpc.GetTriggersRequest) (*pulumirpc.GetTriggersResponse, error)
	GetTrigger(context.Context, *pulumirpc.GetTriggerRequest) (*pulumirpc.GetTriggerResponse, error)
	GetJobs(context.Context, *pulumirpc.GetJobsRequest) (*pulumirpc.GetJobsResponse, error)
	GetJob(context.Context, *pulumirpc.GetJobRequest) (*pulumirpc.GetJobResponse, error)
	GetSteps(context.Context, *pulumirpc.GetStepsRequest) (*pulumirpc.GetStepsResponse, error)
	GetStep(context.Context, *pulumirpc.GetStepRequest) (*pulumirpc.GetStepResponse, error)
	GenerateGraph(context.Context, *pulumirpc.GenerateGraphRequest) (*pulumirpc.GenerateNodeResponse, error)
	GenerateJob(context.Context, *pulumirpc.GenerateJobRequest) (*pulumirpc.GenerateNodeResponse, error)
	RunTriggerMock(context.Context, *pulumirpc.RunTriggerMockRequest) (*pulumirpc.RunTriggerMockResponse, error)
	RunFilter(context.Context, *pulumirpc.RunFilterRequest) (*pulumirpc.RunFilterResponse, error)
	RunStep(context.Context, *pulumirpc.RunStepRequest) (*pulumirpc.RunStepResponse, error)
	RunOnError(context.Context, *pulumirpc.RunOnErrorRequest) (*pulumirpc.RunOnErrorResponse, error)
	ResolveJobResult(context.Context, *pulumirpc.ResolveJobResultRequest) (*pulumirpc.ResolveJobResultResponse, error)
}
