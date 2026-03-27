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

package tests

import (
	"context"
	"net"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

func init() {
	LanguageTests["workflow-external-step-job"] = LanguageTest{
		Workflows: []func() plugin.Workflow{
			func() plugin.Workflow {
				return &plugin.MockWorkflow{
					GetPackageInfoF: func(
						_ context.Context, _ *pulumirpc.EmptyRequest,
					) (*pulumirpc.GetPackageInfoResponse, error) {
						return &pulumirpc.GetPackageInfoResponse{
							Package: &pulumirpc.PackageInfo{
								Name:    "external",
								Version: "1.0.0",
							},
						}, nil
					},
					GetGraphsF: func(
						_ context.Context, _ *pulumirpc.EmptyRequest,
					) (*pulumirpc.GetGraphsResponse, error) {
						return &pulumirpc.GetGraphsResponse{}, nil
					},
					GetTriggersF: func(
						_ context.Context, _ *pulumirpc.EmptyRequest,
					) (*pulumirpc.GetTriggersResponse, error) {
						return &pulumirpc.GetTriggersResponse{}, nil
					},
					GetJobsF: func(
						_ context.Context, _ *pulumirpc.EmptyRequest,
					) (*pulumirpc.GetJobsResponse, error) {
						return &pulumirpc.GetJobsResponse{}, nil
					},
				}
			},
		},
		Runs: []TestRun{
			{
				AssertWorkflow: func(l *L, args AssertWorkflowArgs) {
					jobs, err := args.Workflow.GetJobs(args.Context, &pulumirpc.EmptyRequest{})
					require.NoError(l, err)
					require.Len(l, jobs.GetJobs(), 1)

					jobToken := jobs.GetJobs()[0].GetToken()
					job, err := args.Workflow.GetJob(args.Context, &pulumirpc.TokenLookupRequest{Token: jobToken})
					require.NoError(l, err)
					require.NotNil(l, job.GetJob())
					require.NotNil(l, job.GetJob().GetInputType().GetObject())
					require.Contains(l, job.GetJob().GetInputType().GetObject().GetProperties(), "input")
					assert.Equal(l, "bool", job.GetJob().GetInputType().GetObject().GetProperties()["input"].GetType())

					monitor := &workflowJobMonitor{}
					grpcServer := grpc.NewServer()
					pulumirpc.RegisterGraphMonitorServer(grpcServer, monitor)

					listener, err := net.Listen("tcp4", "127.0.0.1:0")
					require.NoError(l, err)
					defer func() {
						_ = listener.Close()
						grpcServer.Stop()
					}()
					go func() { _ = grpcServer.Serve(listener) }()

					workflowContext := &pulumirpc.WorkflowContext{ExecutionId: "test"}
					generateResp, err := args.Workflow.GenerateJob(args.Context, &pulumirpc.GenerateJobRequest{
						Context:             workflowContext,
						Name:                jobToken,
						GraphMonitorAddress: listener.Addr().String(),
						InputValue: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"input": structpb.NewBoolValue(true),
							},
						},
					})
					require.NoError(l, err)
					require.Empty(l, generateResp.GetError().GetReason())

					steps := monitor.snapshotStepsForJob(jobToken)
					require.Len(l, steps, 1)

					filterResp, err := args.Workflow.RunFilter(args.Context, &pulumirpc.RunFilterRequest{
						Path: steps[0],
					})
					require.NoError(l, err)
					require.True(l, filterResp.GetPass())

					// Feed the job input struct to the external step.
					runResp, err := args.Workflow.RunStep(args.Context, &pulumirpc.RunStepRequest{
						Context: workflowContext,
						Path:    steps[0],
						Input: structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"input": structpb.NewBoolValue(true),
							},
						}),
					})
					require.NoError(l, err)
					require.Empty(l, runResp.GetError().GetReason())
					assert.False(l, runResp.GetResult().GetBoolValue())

					result, err := args.Workflow.ResolveJobResult(args.Context, &pulumirpc.ResolveJobResultRequest{
						Context: workflowContext,
						Path:    jobToken,
					})
					require.NoError(l, err)
					require.Empty(l, result.GetError().GetReason())
					require.NotNil(l, result.GetResult())
					assert.False(l, result.GetResult().GetBoolValue())
				},
			},
		},
	}
}
