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

package workflows

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type SimpleWorkflow struct{}

type simpleWorkflow struct{}

var _ plugin.Workflow = (*simpleWorkflow)(nil)

func (w *SimpleWorkflow) New() plugin.Workflow {
	return &simpleWorkflow{}
}

func workflowNotImplemented(method string) error {
	return fmt.Errorf("%s not implemented", method)
}

func (w *simpleWorkflow) Close() error {
	return nil
}

func (w *simpleWorkflow) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return nil, workflowNotImplemented("Handshake")
}

func (w *simpleWorkflow) GetPackageInfo(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetPackageInfoResponse, error) {
	return &pulumirpc.GetPackageInfoResponse{
		Package: &pulumirpc.PackageInfo{
			Name:    "external",
			Version: "1.0.0",
		},
	}, nil
}

func (w *simpleWorkflow) GetGraphs(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetGraphsResponse, error) {
	return &pulumirpc.GetGraphsResponse{}, nil
}

func (w *simpleWorkflow) GetGraph(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetGraphResponse, error) {
	return nil, workflowNotImplemented("GetGraph")
}

func (w *simpleWorkflow) GetTriggers(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetTriggersResponse, error) {
	return &pulumirpc.GetTriggersResponse{}, nil
}

func (w *simpleWorkflow) GetTrigger(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetTriggerResponse, error) {
	return nil, workflowNotImplemented("GetTrigger")
}

func (w *simpleWorkflow) GetJobs(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetJobsResponse, error) {
	return &pulumirpc.GetJobsResponse{}, nil
}

func (w *simpleWorkflow) GetJob(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetJobResponse, error) {
	return nil, workflowNotImplemented("GetJob")
}

func (w *simpleWorkflow) GetSteps(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetStepsResponse, error) {
	return nil, workflowNotImplemented("GetSteps")
}

func (w *simpleWorkflow) GetStep(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetStepResponse, error) {
	return nil, workflowNotImplemented("GetStep")
}

func (w *simpleWorkflow) GenerateGraph(
	context.Context, *pulumirpc.GenerateGraphRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return nil, workflowNotImplemented("GenerateGraph")
}

func (w *simpleWorkflow) GenerateJob(
	context.Context, *pulumirpc.GenerateJobRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return nil, workflowNotImplemented("GenerateJob")
}

func (w *simpleWorkflow) RunTriggerMock(
	context.Context, *pulumirpc.RunTriggerMockRequest,
) (*pulumirpc.RunTriggerMockResponse, error) {
	return nil, workflowNotImplemented("RunTriggerMock")
}

func (w *simpleWorkflow) RunFilter(
	context.Context, *pulumirpc.RunFilterRequest,
) (*pulumirpc.RunFilterResponse, error) {
	return nil, workflowNotImplemented("RunFilter")
}

func (w *simpleWorkflow) RunStep(
	context.Context, *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	return nil, workflowNotImplemented("RunStep")
}

func (w *simpleWorkflow) RunOnError(
	context.Context, *pulumirpc.RunOnErrorRequest,
) (*pulumirpc.RunOnErrorResponse, error) {
	return nil, workflowNotImplemented("RunOnError")
}

func (w *simpleWorkflow) ResolveJobResult(
	context.Context, *pulumirpc.ResolveJobResultRequest,
) (*pulumirpc.ResolveJobResultResponse, error) {
	return nil, workflowNotImplemented("ResolveJobResult")
}
