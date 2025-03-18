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
	"gopkg.in/yaml.v2"
)

type L1EmptyLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
	// If true we won't create the stack resource in Run.
	skipStack bool
	// If true then we'll fail the pack command (which is only used for the core SDK for l1-empty)
	failPack bool
	// If true behave as if there's no core SDK.
	skipCoreSDK bool
}

func (h *L1EmptyLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	if !strings.HasSuffix(req.PackageDirectory, "/sdk/dir") {
		return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
	}

	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	if h.failPack {
		return nil, errors.New("boom")
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
	}, nil
}

func (h *L1EmptyLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if !h.skipCoreSDK {
		if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
			return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
		}
	}
	if !req.Strict {
		return nil, errors.New("expected strict to be true")
	}
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "l1-empty") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l1-empty" {
		return nil, fmt.Errorf("unexpected project name %s", project.Name)
	}
	project.Runtime = workspace.NewProjectRuntimeInfo("mock", nil)
	projectYaml, err := yaml.Marshal(project)
	if err != nil {
		return nil, fmt.Errorf("could not marshal project: %w", err)
	}

	// Write the minimal project file.
	if err := os.WriteFile(filepath.Join(req.TargetDirectory, "Pulumi.yaml"), projectYaml, 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (h *L1EmptyLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l1-empty") {
		return nil, fmt.Errorf("unexpected directory to get program dependencies %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
			// Return some other random dependency to make sure we can handle it.
			{
				Name:    "random_dep",
				Version: "0.4.0",
			},
		},
	}, nil
}

func (h *L1EmptyLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l1-empty") {
		return fmt.Errorf("unexpected root directory to install dependencies %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
		return fmt.Errorf("unexpected program directory to install dependencies %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return fmt.Errorf("unexpected entry point to install dependencies %s", req.Info.EntryPoint)
	}
	return nil
}

func (h *L1EmptyLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l1-empty") {
		return nil, fmt.Errorf("unexpected root directory to run %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
		return nil, fmt.Errorf("unexpected program directory to run %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return nil, fmt.Errorf("unexpected entry point to run %s", req.Info.EntryPoint)
	}

	if !h.skipStack {
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
	}
	return &pulumirpc.RunResponse{}, nil
}

// Run a simple successful test with a mocked runtime.
func TestL1Empty(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	runtime := &L1EmptyLanguageHost{tempDir: tempDir}
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
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.True(t, runResponse.Success)
	assert.Empty(t, runResponse.Messages)
}

// Test simple failure conditions for Prepare.
func TestL1Empty_FailPrepare(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	runtime := &L1EmptyLanguageHost{
		tempDir:  tempDir,
		failPack: true,
	}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, runtime)
			return nil
		},
	})
	require.NoError(t, err)

	t.Run("missing plugin name", func(t *testing.T) {
		t.Parallel()

		_, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
			LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
			TemporaryDirectory:   tempDir,
			SnapshotDirectory:    "./tests/testdata/snapshots",
			CoreSdkDirectory:     "sdk/dir",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "language plugin name must be specified")
	})

	t.Run("missing plugin target", func(t *testing.T) {
		t.Parallel()

		_, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
			LanguagePluginName: "mock",
			TemporaryDirectory: tempDir,
			SnapshotDirectory:  "./tests/testdata/snapshots",
			CoreSdkDirectory:   "sdk/dir",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "language plugin target must be specified")
	})

	t.Run("missing temporary directory", func(t *testing.T) {
		t.Parallel()

		_, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
			LanguagePluginName:   "mock",
			LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
			SnapshotDirectory:    "./tests/testdata/snapshots",
			CoreSdkDirectory:     "sdk/dir",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "temporary directory must be specified")
	})

	t.Run("missing snapshot directory", func(t *testing.T) {
		t.Parallel()

		_, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
			LanguagePluginName:   "mock",
			LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
			TemporaryDirectory:   tempDir,
			CoreSdkDirectory:     "sdk/dir",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "snapshot directory must be specified")
	})

	t.Run("fail packing of core sdk", func(t *testing.T) {
		t.Parallel()

		_, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
			LanguagePluginName:   "mock",
			LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
			TemporaryDirectory:   tempDir,
			SnapshotDirectory:    "./tests/testdata/snapshots",
			CoreSdkDirectory:     "sdk/dir",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "pack core SDK: boom")
	})
}

// Run a simple failing test because of a bad project snapshot with a mocked runtime.
func TestL1Empty_BadSnapshot(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := &languageTestServer{DisableSnapshotWriting: true}
	runtime := &L1EmptyLanguageHost{tempDir: tempDir}
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
		SnapshotDirectory:    "./tests/testdata/snapshots_bad",
		CoreSdkDirectory:     "sdk/dir",
		CoreSdkVersion:       "1.0.1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	assert.Contains(t, runResponse.Messages[0],
		"program snapshot validation failed:\nexpected file Pulumi.yaml does not match actual file")
}

// Run a simple failing test because of a bad project snapshot with a mocked runtime.
func TestL1Empty_MissingStack(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	runtime := &L1EmptyLanguageHost{
		tempDir:   tempDir,
		skipStack: true,
	}
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
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	failureMessage := runResponse.Messages[0]
	assert.Contains(t, failureMessage, "expected at least 1 StepOp")
}

// Run a simple passing test for a language without a core SDK (e.g. yaml)
func TestL1Empty_NoCoreSDK(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	runtime := &L1EmptyLanguageHost{
		tempDir:     tempDir,
		skipStack:   true,
		skipCoreSDK: true,
	}
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
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	failureMessage := runResponse.Messages[0]
	assert.Contains(t, failureMessage, "expected at least 1 StepOp")
}
