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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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

type L2ResourceSimpleLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
	// Skip creating the simple resource in Run
	skipResource bool
	// Skip returning the simple resource plugin in GetRequiredPlugins
	skipRequiredPlugins bool
	// Used by the language info test to assert we see language info in the schema.
	expectLanguageInfo bool
}

func (h *L2ResourceSimpleLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "simple-2.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "simple-2.0.0.sdk"),
		}, nil
	} else if req.PackageDirectory != filepath.Join(h.tempDir, "sdks", "core") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
		}, nil
	}

	return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
}

func (h *L2ResourceSimpleLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.LocalDependencies["simple"] != filepath.Join(h.tempDir, "artifacts", "simple-2.0.0.sdk") {
		return nil, fmt.Errorf("unexpected simple sdk %s", req.LocalDependencies["simple"])
	}
	if !req.Strict {
		return nil, errors.New("expected strict to be true")
	}
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l2-resource-simple" {
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

func (h *L2ResourceSimpleLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.Directory != filepath.Join(h.tempDir, "sdks", "simple-2.0.0") {
		return nil, fmt.Errorf("unexpected directory %s", req.Directory)
	}

	if h.expectLanguageInfo {
		// Check we see the language info block in the schema.
		var spec map[string]interface{}
		err := json.Unmarshal([]byte(req.Schema), &spec)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal schema: %w", err)
		}
		languageRaw, ok := spec["language"]
		if !ok {
			return nil, errors.New("expected language block in schema")
		}
		language, ok := languageRaw.(map[string]interface{})
		if !ok {
			return nil, errors.New("expected language block in schema to be a map")
		}
		infoRaw, ok := language["mock"]
		if !ok {
			return nil, errors.New("expected mock language block in schema")
		}
		info, ok := infoRaw.(map[string]interface{})
		if !ok {
			return nil, errors.New("expected mock language block in schema to be a map")
		}
		if info["name"] != "mock" {
			return nil, fmt.Errorf("unexpected language name %s", info["name"])
		}
	}

	// Write the minimal package code.
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (h *L2ResourceSimpleLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
		return nil, fmt.Errorf("unexpected directory to get required plugins %s", req.Info.ProgramDirectory)
	}

	if h.skipRequiredPlugins {
		return &pulumirpc.GetRequiredPluginsResponse{}, nil
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: []*pulumirpc.PluginDependency{
			{
				Name:    "simple",
				Kind:    string(apitype.ResourcePlugin),
				Version: "2.0.0",
			},
		},
	}, nil
}

func (h *L2ResourceSimpleLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
		return nil, fmt.Errorf("unexpected directory to get program dependencies %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
			{
				Name:    "pulumi_simple",
				Version: "2.0.0",
			},
		},
	}, nil
}

func (h *L2ResourceSimpleLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
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

func (h *L2ResourceSimpleLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
		return nil, fmt.Errorf("unexpected root directory to run %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
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

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}

	if !h.skipResource {
		_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:   "simple:index:Resource",
			Custom: true,
			Name:   "res",
			Object: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"value": structpb.NewBoolValue(true),
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
	}

	return &pulumirpc.RunResponse{}, nil
}

// Run a simple successful test with a mocked runtime.
func TestL2ResourceSimple(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceSimpleLanguageHost{tempDir: tempDir}
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
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}

// Run a simple failing test because of a bad sdk snapshot with a mocked runtime.
func TestL2SimpleResource_BadSnapshot(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	engine.DisableSnapshotWriting = true
	runtime := &L2ResourceSimpleLanguageHost{tempDir: tempDir}
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
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	assert.Contains(t, runResponse.Messages[0],
		"sdk snapshot validation for simple failed:\nexpected file test.txt does not match actual file")
}

// Run a simple failing test because of a bad project snapshot with a mocked runtime.
func TestL2SimpleResource_MissingResource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceSimpleLanguageHost{
		tempDir:      tempDir,
		skipResource: true,
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
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	failureMessage := runResponse.Messages[0]
	assert.Contains(t, failureMessage, "expected 3 resources in snapshot")
}

// Run a simple failing test because GetRequiredPlugins doesn't return the right plugins.
func TestL2SimpleResource_MissingRequiredPlugins(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceSimpleLanguageHost{
		tempDir:             tempDir,
		skipRequiredPlugins: true,
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
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.False(t, runResponse.Success)
	require.Len(t, runResponse.Messages, 1)
	failureMessage := runResponse.Messages[0]
	assert.Contains(t, failureMessage, "missing expected package simple-2.0.0")
}

// Run a simple successful test with a mocked runtime that edits the snapshot files.
func TestL2ResourceSnapshotEdit(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceSimpleLanguageHost{tempDir: tempDir}
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
		SnapshotDirectory:    "./tests/testdata/snapshots_edit",
		CoreSdkDirectory:     "sdk/dir",
		SnapshotEdits: []*testingrpc.PrepareLanguageTestsRequest_Replacement{
			{
				Path:        "test.txt",
				Pattern:     "testing",
				Replacement: "replaced",
			},
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}

// Run a simple successful test with a mocked runtime that wants a language info block.
func TestL2ResourceLanguageInfo(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceSimpleLanguageHost{
		tempDir:            tempDir,
		expectLanguageInfo: true,
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
		LanguageInfo:         "{\"name\":\"mock\"}",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l2-resource-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
