// Copyright 2025, Pulumi Corporation.
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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProgramOverridesLanguageHost is a mock language host designed to exercise program overrides. It is only capable of
// passing the "l1-empty" conformance test (which simply registers a stack resource).
type ProgramOverridesLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	generateProjectCalled bool
	runCalled             bool
}

func (h *ProgramOverridesLanguageHost) GenerateProject(
	_ context.Context,
	req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	h.generateProjectCalled = true

	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (h *ProgramOverridesLanguageHost) Pack(
	_ context.Context,
	req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	return &pulumirpc.PackResponse{}, nil
}

func (h *ProgramOverridesLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (h *ProgramOverridesLanguageHost) GetProgramDependencies(
	_ context.Context,
	req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (h *ProgramOverridesLanguageHost) Run(
	ctx context.Context,
	req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	h.runCalled = true

	conn, err := grpc.NewClient(
		req.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to resource monitor: %w", err)
	}
	defer conn.Close()

	monitor := pulumirpc.NewResourceMonitorClient(conn)

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Tests that a conformance test which specifies program overrides does not ask the language host to generate a project,
// but otherwise behaves as expected (validating snaphots, checking assertions, etc.).
func TestProgramOverrides_DontGenerateProgram(t *testing.T) {
	t.Parallel()

	// Arrange.
	tempDir := t.TempDir()

	ctx := context.Background()
	engine := &languageTestServer{}

	runtime := &ProgramOverridesLanguageHost{}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, runtime)
			return nil
		},
	})
	require.NoError(t, err)

	prepareResponse, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "mock",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   tempDir,
		SnapshotDirectory:    "./tests/testdata/snapshots",
		ProgramOverrides: map[string]*testingrpc.PrepareLanguageTestsRequest_ProgramOverride{
			"l1-empty": {
				Path: "./tests/testdata/overrides/l1-empty",
			},
		},
	})

	require.NoError(t, err)
	require.NotEmpty(t, prepareResponse.Token)

	// Act.
	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l1-empty",
	})

	// Assert.
	require.NoError(t, err)
	require.NotNil(t, runResponse)

	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)

	require.Empty(t, runResponse.Messages)
	require.True(t, runResponse.Success)

	require.False(t, runtime.generateProjectCalled, "GenerateProject should not have been called")
	require.True(t, runtime.runCalled, "Run should have been called")
}
