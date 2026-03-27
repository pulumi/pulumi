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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

type workflowSimpleJobLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir          string
	generateJobError string
}

func (h *workflowSimpleJobLanguageHost) Pack(
	context.Context, *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(h.tempDir, "artifacts", "core.sdk"),
	}, nil
}

func (h *workflowSimpleJobLanguageHost) GenerateProject(
	_ context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if !req.Strict {
		return nil, fmt.Errorf("expected strict generation")
	}

	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "workflow-constant-job") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}

	if err := os.WriteFile(
		filepath.Join(req.TargetDirectory, "PulumiPlugin.yaml"),
		[]byte("runtime: mock\n"),
		0o600,
	); err != nil {
		return nil, err
	}

	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (h *workflowSimpleJobLanguageHost) GetRequiredPlugins(
	context.Context, *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (h *workflowSimpleJobLanguageHost) GetProgramDependencies(
	context.Context, *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (h *workflowSimpleJobLanguageHost) InstallDependencies(
	_ *pulumirpc.InstallDependenciesRequest, _ pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

type workflowSimpleJobPlugin struct {
	pulumirpc.UnimplementedWorkflowEvaluatorServer

	generateJobError string
}

func (p *workflowSimpleJobPlugin) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return &pulumirpc.WorkflowHandshakeResponse{}, nil
}

func (p *workflowSimpleJobPlugin) GetJobs(
	context.Context, *pulumirpc.EmptyRequest,
) (*pulumirpc.GetJobsResponse, error) {
	return &pulumirpc.GetJobsResponse{
		Jobs: []*pulumirpc.JobInfo{
			{Token: "example:index:build"},
		},
	}, nil
}

func (p *workflowSimpleJobPlugin) GetJob(
	_ context.Context, req *pulumirpc.TokenLookupRequest,
) (*pulumirpc.GetJobResponse, error) {
	if req.Token != "example:index:build" {
		return nil, fmt.Errorf("unexpected job token %q", req.Token)
	}
	return &pulumirpc.GetJobResponse{
		Job: &pulumirpc.JobInfo{
			Token: req.Token,
		},
	}, nil
}

func (p *workflowSimpleJobPlugin) GenerateJob(
	ctx context.Context, req *pulumirpc.GenerateJobRequest,
) (*pulumirpc.GenerateNodeResponse, error) {
	if p.generateJobError != "" {
		return &pulumirpc.GenerateNodeResponse{
			Error: &pulumirpc.WorkflowError{Reason: p.generateJobError},
		}, nil
	}

	conn, err := grpc.NewClient(
		req.GetGraphMonitorAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect graph monitor: %w", err)
	}
	defer conn.Close()

	monitor := pulumirpc.NewGraphMonitorClient(conn)
	_, err = monitor.RegisterJob(ctx, &pulumirpc.RegisterJobRequest{
		Path: req.GetName(),
	})
	if err != nil {
		return nil, fmt.Errorf("register job: %w", err)
	}

	return &pulumirpc.GenerateNodeResponse{}, nil
}

func (p *workflowSimpleJobPlugin) RunFilter(
	context.Context, *pulumirpc.RunFilterRequest,
) (*pulumirpc.RunFilterResponse, error) {
	return &pulumirpc.RunFilterResponse{Pass: true}, nil
}

func (p *workflowSimpleJobPlugin) RunStep(
	_ context.Context, req *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	return nil, fmt.Errorf("unexpected step execution %q", req.Path)
}

func (p *workflowSimpleJobPlugin) ResolveJobResult(
	_ context.Context, req *pulumirpc.ResolveJobResultRequest,
) (*pulumirpc.ResolveJobResultResponse, error) {
	if req.Path != "example:index:build" {
		return nil, fmt.Errorf("unexpected resolve path %q", req.Path)
	}

	return &pulumirpc.ResolveJobResultResponse{
		Result: structpb.NewStringValue("done"),
	}, nil
}

func (h *workflowSimpleJobLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	if req.Kind != string(apitype.WorkflowPlugin) {
		return fmt.Errorf("unexpected plugin kind %s", req.Kind)
	}

	workflowPlugin := &workflowSimpleJobPlugin{
		generateJobError: h.generateJobError,
	}
	stop := make(chan bool)
	go func() {
		<-server.Context().Done()
		stop <- true
	}()

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterWorkflowEvaluatorServer(srv, workflowPlugin)
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return fmt.Errorf("could not start workflow plugin: %w", err)
	}

	if err := server.Send(&pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{
			Stdout: []byte(fmt.Sprintf("%v\n", handle.Port)),
		},
	}); err != nil {
		return fmt.Errorf("could not send plugin port: %w", err)
	}

	return <-handle.Done
}

func prepareWorkflowSimpleJobTest(t *testing.T, runtime *workflowSimpleJobLanguageHost) (*languageTestServer, string) {
	t.Helper()
	t.Setenv("PULUMI_ACCEPT", "1")

	if runtime == nil {
		runtime = &workflowSimpleJobLanguageHost{}
	}
	tempDir := t.TempDir()
	runtime.tempDir = tempDir

	engine := newLanguageTestServer()
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, runtime)
			return nil
		},
	})
	require.NoError(t, err)

	prepareResponse, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "mock",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   tempDir,
		SnapshotDirectory:    t.TempDir(),
		CoreSdkDirectory:     "sdk/dir",
		CoreSdkVersion:       "1.0.1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, prepareResponse.Token)

	return engine, prepareResponse.Token
}

func TestWorkflowSimpleJob(t *testing.T) {
	engine, token := prepareWorkflowSimpleJobTest(t, nil)

	runResponse, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
		Token: token,
		Test:  "workflow-constant-job",
	})
	require.NoError(t, err)
	assert.True(t, runResponse.Success)
	assert.Empty(t, runResponse.Messages)
}

func TestWorkflowSimpleJob_GenerateJobError(t *testing.T) {
	engine, token := prepareWorkflowSimpleJobTest(t, &workflowSimpleJobLanguageHost{
		generateJobError: "boom from generate job",
	})

	runResponse, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
		Token: token,
		Test:  "workflow-constant-job",
	})
	require.NoError(t, err)
	assert.False(t, runResponse.Success)
	require.NotEmpty(t, runResponse.Messages)
	assert.Contains(t, runResponse.Messages[0], "boom from generate job")
}
