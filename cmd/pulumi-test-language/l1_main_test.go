// Copyright 2016-2023, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v3"
)

type L1MainLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
}

func (h *L1MainLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	if !strings.HasSuffix(req.PackageDirectory, "/sdk/dir") {
		return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
	}

	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
	}, nil
}

func (h *L1MainLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if !req.Strict {
		return nil, errors.New("expected strict to be true")
	}
	if filepath.Base(req.SourceDirectory) != "subdir" {
		return nil, fmt.Errorf("unexpected source directory %s", req.SourceDirectory)
	}
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "l1-main") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l1-main" {
		return nil, fmt.Errorf("unexpected project name %s", project.Name)
	}
	if project.Main != "subdir" {
		return nil, fmt.Errorf("unexpected project main %s", project.Main)
	}
	project.Runtime = workspace.NewProjectRuntimeInfo("mock", nil)
	projectYaml, err := yaml.Marshal(project)
	if err != nil {
		return nil, fmt.Errorf("marshal project: %w", err)
	}

	// Write the minimal project file.
	if err := os.WriteFile(filepath.Join(req.TargetDirectory, "Pulumi.yaml"), projectYaml, 0o600); err != nil {
		return nil, fmt.Errorf("write project file: %w", err)
	}
	// And the main subdir, although nothing is in it
	if err := os.MkdirAll(filepath.Join(req.TargetDirectory, "subdir"), 0o700); err != nil {
		return nil, fmt.Errorf("make main directory: %w", err)
	}

	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (h *L1MainLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l1-main") {
		return fmt.Errorf("unexpected root directory to install dependencies %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l1-main", "subdir") {
		return fmt.Errorf("unexpected program directory to install dependencies %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return fmt.Errorf("unexpected entry point to install dependencies %s", req.Info.EntryPoint)
	}
	return nil
}

func (h *L1MainLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l1-main") {
		return nil, fmt.Errorf("unexpected root directory to get program dependencies %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l1-main", "subdir") {
		return nil, fmt.Errorf("unexpected program directory to get program dependencies %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return nil, fmt.Errorf("unexpected entry point to get program dependencies %s", req.Info.EntryPoint)
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
		},
	}, nil
}

func (h *L1MainLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l1-main") {
		return nil, fmt.Errorf("unexpected root directory to run %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l1-main", "subdir") {
		return nil, fmt.Errorf("unexpected program directory to run %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return nil, fmt.Errorf("unexpected entry point to run %s", req.Info.EntryPoint)
	}

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

	resp, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}

	outputs, err := structpb.NewStruct(map[string]interface{}{
		"output_true": true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not marshal outputs: %w", err)
	}

	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     resp.Urn,
		Outputs: outputs,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register outputs: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Run a simple successful test
func TestL1Main(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L1MainLanguageHost{tempDir: tempDir}
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
		CoreSdkDirectory:     "sdk/dir",
		CoreSdkVersion:       "1.0.1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l1-main",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.True(t, runResponse.Success)
	assert.Empty(t, runResponse.Messages)
}
