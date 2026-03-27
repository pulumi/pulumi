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
	"errors"
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
	"google.golang.org/protobuf/types/known/structpb"
)

type workflowSimpleStepLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir           string
	getStepsError     string
	returnInputResult bool
}

func (h *workflowSimpleStepLanguageHost) Pack(
	context.Context, *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(h.tempDir, "artifacts", "core.sdk"),
	}, nil
}

func (h *workflowSimpleStepLanguageHost) GenerateProject(
	_ context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if !req.Strict {
		return nil, fmt.Errorf("expected strict generation")
	}

	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "workflow-simple-step") {
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

func (h *workflowSimpleStepLanguageHost) GetRequiredPlugins(
	context.Context, *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (h *workflowSimpleStepLanguageHost) GetProgramDependencies(
	context.Context, *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (h *workflowSimpleStepLanguageHost) InstallDependencies(
	_ *pulumirpc.InstallDependenciesRequest, _ pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

type workflowSimpleStepPlugin struct {
	pulumirpc.UnimplementedWorkflowEvaluatorServer

	getStepsError     string
	returnInputResult bool
}

func (p *workflowSimpleStepPlugin) Handshake(
	context.Context, *pulumirpc.WorkflowHandshakeRequest,
) (*pulumirpc.WorkflowHandshakeResponse, error) {
	return &pulumirpc.WorkflowHandshakeResponse{}, nil
}

func (p *workflowSimpleStepPlugin) GetSteps(
	context.Context, *pulumirpc.GetStepsRequest,
) (*pulumirpc.GetStepsResponse, error) {
	if p.getStepsError != "" {
		return nil, errors.New(p.getStepsError)
	}

	return &pulumirpc.GetStepsResponse{
		Steps: []string{"echo"},
	}, nil
}

func (p *workflowSimpleStepPlugin) GetStep(
	_ context.Context, req *pulumirpc.GetStepRequest,
) (*pulumirpc.GetStepResponse, error) {
	if req.Token != "echo" {
		return nil, fmt.Errorf("unexpected step token %q", req.Token)
	}
	return &pulumirpc.GetStepResponse{
		InputType:  &pulumirpc.TypeReference{Token: "bool"},
		OutputType: &pulumirpc.TypeReference{Token: "bool"},
	}, nil
}

func (p *workflowSimpleStepPlugin) RunStep(
	_ context.Context, req *pulumirpc.RunStepRequest,
) (*pulumirpc.RunStepResponse, error) {
	if req.Path != "echo" {
		return nil, fmt.Errorf("unexpected step path %q", req.Path)
	}
	input := req.GetInput()
	if input == nil {
		return nil, fmt.Errorf("missing step input")
	}

	output := !input.GetBoolValue()
	if p.returnInputResult {
		output = input.GetBoolValue()
	}

	return &pulumirpc.RunStepResponse{
		Result: structpb.NewBoolValue(output),
	}, nil
}

func (h *workflowSimpleStepLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	if req.Kind != string(apitype.WorkflowPlugin) {
		return fmt.Errorf("unexpected plugin kind %s", req.Kind)
	}

	workflowPlugin := &workflowSimpleStepPlugin{
		getStepsError:     h.getStepsError,
		returnInputResult: h.returnInputResult,
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

func prepareWorkflowSimpleStepTest(t *testing.T, runtime *workflowSimpleStepLanguageHost) (*languageTestServer, string) {
	t.Helper()

	t.Setenv("PULUMI_ACCEPT", "1")

	if runtime == nil {
		runtime = &workflowSimpleStepLanguageHost{}
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

func TestWorkflowSimpleStep(t *testing.T) {
	engine, token := prepareWorkflowSimpleStepTest(t, nil)

	runResponse, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
		Token: token,
		Test:  "workflow-simple-step",
	})
	require.NoError(t, err)
	assert.True(t, runResponse.Success)
	assert.Empty(t, runResponse.Messages)
}

func TestWorkflowSimpleStep_GetStepsError(t *testing.T) {
	engine, token := prepareWorkflowSimpleStepTest(t, &workflowSimpleStepLanguageHost{
		getStepsError: "boom from workflow evaluator",
	})

	runResponse, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
		Token: token,
		Test:  "workflow-simple-step",
	})
	require.NoError(t, err)
	assert.False(t, runResponse.Success)
	require.NotEmpty(t, runResponse.Messages)
	assert.Contains(t, runResponse.Messages[0], "boom from workflow evaluator")
}

func TestWorkflowSimpleStep_RunStepUnexpectedResult(t *testing.T) {
	engine, token := prepareWorkflowSimpleStepTest(t, &workflowSimpleStepLanguageHost{
		returnInputResult: true,
	})

	runResponse, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
		Token: token,
		Test:  "workflow-simple-step",
	})
	require.NoError(t, err)
	assert.False(t, runResponse.Success)
	require.NotEmpty(t, runResponse.Messages)
	assert.Contains(t, runResponse.Messages[0], "Should be false")
}
