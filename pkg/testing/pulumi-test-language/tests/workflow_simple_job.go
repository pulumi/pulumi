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
	"strings"
	"sync"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type workflowJobMonitor struct {
	pulumirpc.UnimplementedGraphMonitorServer

	mu    sync.Mutex
	steps []string
}

func (m *workflowJobMonitor) RegisterGraph(
	context.Context, *pulumirpc.RegisterGraphRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *workflowJobMonitor) RegisterTrigger(
	context.Context, *pulumirpc.RegisterTriggerRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *workflowJobMonitor) RegisterSensor(
	context.Context, *pulumirpc.RegisterSensorRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *workflowJobMonitor) RegisterJob(
	context.Context, *pulumirpc.RegisterJobRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *workflowJobMonitor) RegisterStep(
	_ context.Context, req *pulumirpc.RegisterStepRequest,
) (*pulumirpc.RegisterNodeResponse, error) {
	stepPath := req.GetJob() + "/steps/" + req.GetName()

	m.mu.Lock()
	m.steps = append(m.steps, stepPath)
	m.mu.Unlock()

	return &pulumirpc.RegisterNodeResponse{}, nil
}

func (m *workflowJobMonitor) snapshotStepsForJob(jobPath string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	steps := make([]string, 0, len(m.steps))
	prefix := jobPath + "/steps/"
	for _, step := range m.steps {
		if strings.HasPrefix(step, prefix) {
			steps = append(steps, step)
		}
	}
	return steps
}

func init() {
	LanguageTests["workflow-simple-job"] = LanguageTest{
		Runs: []TestRun{
			{
				AssertWorkflow: func(l *L, args AssertWorkflowArgs) {
					jobs, err := args.Workflow.GetJobs(args.Context, &pulumirpc.GetJobsRequest{})
					require.NoError(l, err)
					require.Len(l, jobs.GetJobs(), 1)

					jobToken := jobs.GetJobs()[0].GetToken()
					job, err := args.Workflow.GetJob(args.Context, &pulumirpc.GetJobRequest{Token: jobToken})
					require.NoError(l, err)
					require.NotNil(l, job.GetJob())

					monitor := &workflowJobMonitor{}
					grpcServer := grpc.NewServer()
					pulumirpc.RegisterGraphMonitorServer(grpcServer, monitor)

					listener, err := net.Listen("tcp4", "127.0.0.1:0")
					require.NoError(l, err)
					defer func() {
						_ = listener.Close()
						grpcServer.Stop()
					}()

					go func() {
						_ = grpcServer.Serve(listener)
					}()

					workflowContext := &pulumirpc.WorkflowContext{ExecutionId: "test"}
					generateResp, err := args.Workflow.GenerateJob(args.Context, &pulumirpc.GenerateJobRequest{
						Context:             workflowContext,
						Name:                jobToken,
						GraphMonitorAddress: listener.Addr().String(),
					})
					require.NoError(l, err)
					require.Empty(l, generateResp.GetError().GetReason())

					steps := monitor.snapshotStepsForJob(jobToken)
					require.NotEmpty(l, steps)

					for _, step := range steps {
						filterResp, err := args.Workflow.RunFilter(args.Context, &pulumirpc.RunFilterRequest{
							Path: step,
						})
						require.NoError(l, err)
						if !filterResp.GetPass() {
							continue
						}

						runResp, err := args.Workflow.RunStep(args.Context, &pulumirpc.RunStepRequest{
							Context: workflowContext,
							Path:    step,
						})
						require.NoError(l, err)
						require.Empty(l, runResp.GetError().GetReason())
					}

					result, err := args.Workflow.ResolveJobResult(args.Context, &pulumirpc.ResolveJobResultRequest{
						Context: workflowContext,
						Path:    jobToken,
					})
					require.NoError(l, err)
					require.Empty(l, result.GetError().GetReason())
					require.NotNil(l, result.GetResult())
					assert.Equal(l, "done", result.GetResult().GetStringValue())
				},
			},
		},
	}
}
