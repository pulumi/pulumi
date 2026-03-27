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

type SimpleStepWorkflow struct{}

var _ plugin.Workflow = (*SimpleStepWorkflow)(nil)

func workflowNotImplemented(method string) error {
	return fmt.Errorf("%s not implemented", method)
}

func (w *SimpleStepWorkflow) Close() error {
	return nil
}

func (w *SimpleStepWorkflow) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return nil, workflowNotImplemented("Handshake")
}

func (w *SimpleStepWorkflow) GetPackageInfo(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetPackageInfoResponse, error) {
	return &pulumirpc.GetPackageInfoResponse{
		Package: &pulumirpc.PackageInfo{
			Name:    "external",
			Version: "1.0.0",
		},
	}, nil
}

func (w *SimpleStepWorkflow) GetGraphs(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetGraphsResponse, error) {
	return &pulumirpc.GetGraphsResponse{}, nil
}

func (w *SimpleStepWorkflow) GetGraph(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetGraphResponse, error) {
	return nil, workflowNotImplemented("GetGraph")
}

func (w *SimpleStepWorkflow) GetTriggers(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetTriggersResponse, error) {
	return &pulumirpc.GetTriggersResponse{}, nil
}

func (w *SimpleStepWorkflow) GetTrigger(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetTriggerResponse, error) {
	return nil, workflowNotImplemented("GetTrigger")
}

func (w *SimpleStepWorkflow) GetJobs(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetJobsResponse, error) {
	return &pulumirpc.GetJobsResponse{}, nil
}

func (w *SimpleStepWorkflow) GetJob(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetJobResponse, error) {
	return nil, workflowNotImplemented("GetJob")
}

func (w *SimpleStepWorkflow) GetSteps(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetStepsResponse, error) {
	return nil, workflowNotImplemented("GetSteps")
}

func (w *SimpleStepWorkflow) GetStep(
	context.Context, *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetStepResponse, error) {
	return nil, workflowNotImplemented("GetStep")
}

func (w *SimpleStepWorkflow) GenerateGraph(
	context.Context, *pulumirpc.GenerateGraphRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return nil, workflowNotImplemented("GenerateGraph")
}

func (w *SimpleStepWorkflow) GenerateJob(
	context.Context, *pulumirpc.GenerateJobRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	return nil, workflowNotImplemented("GenerateJob")
}

func (w *SimpleStepWorkflow) RunTriggerMock(
	context.Context, *pulumirpc.RunTriggerMockRequest,
) (*pulumirpc.RunTriggerMockResponse, error) {
	return nil, workflowNotImplemented("RunTriggerMock")
}

func (w *SimpleStepWorkflow) RunFilter(
	context.Context, *pulumirpc.RunFilterRequest,
) (*pulumirpc.RunFilterResponse, error) {
	return nil, workflowNotImplemented("RunFilter")
}

func (w *SimpleStepWorkflow) RunStep(
	context.Context, *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	return nil, workflowNotImplemented("RunStep")
}

func (w *SimpleStepWorkflow) RunOnError(
	context.Context, *pulumirpc.RunOnErrorRequest,
) (*pulumirpc.RunOnErrorResponse, error) {
	return nil, workflowNotImplemented("RunOnError")
}

func (w *SimpleStepWorkflow) ResolveJobResult(
	context.Context, *pulumirpc.ResolveJobResultRequest,
) (*pulumirpc.ResolveJobResultResponse, error) {
	return nil, workflowNotImplemented("ResolveJobResult")
}
