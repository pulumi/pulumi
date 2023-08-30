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
	"gopkg.in/yaml.v2"
)

type L2ResourceSimpleLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
	// Skip creating the simple resource in Run
	skipResource bool
	// Skip returning the simple resource plugin in GetRequiredPlugins
	skipRequiredPlugins bool
}

func (h *L2ResourceSimpleLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if !strings.HasSuffix(req.PackageDirectory, "/sdk/dir") &&
		req.PackageDirectory != filepath.Join(h.tempDir, "sdks", "simple-1.0.0") {
		return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
	}

	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	if req.Version != "1.0.0" {
		return nil, fmt.Errorf("unexpected version %s", req.Version)
	}

	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "simple-1.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "simple-1.0.0.sdk"),
		}, nil
	}

	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
	}, nil
}

func (h *L2ResourceSimpleLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.LocalDependencies["simple"] != filepath.Join(h.tempDir, "artifacts", "simple-1.0.0.sdk") {
		return nil, fmt.Errorf("unexpected simple sdk %s", req.LocalDependencies["simple"])
	}
	if !req.Strict {
		return nil, fmt.Errorf("expected strict to be true")
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
	if req.Directory != filepath.Join(h.tempDir, "sdks", "simple-1.0.0") {
		return nil, fmt.Errorf("unexpected directory %s", req.Directory)
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
	if h.skipRequiredPlugins {
		return &pulumirpc.GetRequiredPluginsResponse{}, nil
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: []*pulumirpc.PluginDependency{
			{
				Name:    "simple",
				Kind:    string(workspace.ResourcePlugin),
				Version: "1.0.0",
			},
		},
	}, nil
}

func (h *L2ResourceSimpleLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Directory != filepath.Join(h.tempDir, "projects", "l2-resource-simple") {
		return fmt.Errorf("unexpected directory to install dependencies %s", req.Directory)
	}
	return nil
}

func (h *L2ResourceSimpleLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	conn, err := grpc.Dial(
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

	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
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
		SnapshotDirectory:    "./testdata/snapshots",
		CoreSdkDirectory:     "sdk/dir",
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

	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
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
		SnapshotDirectory:    "./testdata/snapshots_bad",
		CoreSdkDirectory:     "sdk/dir",
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
	assert.Equal(t, []string{
		"sdk snapshot validation for simple failed:\nexpected file test.txt does not match actual file",
	}, runResponse.Messages)
}

// Run a simple failing test because of a bad project snapshot with a mocked runtime.
func TestL2SimpleResource_MissingResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
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
		SnapshotDirectory:    "./testdata/snapshots",
		CoreSdkDirectory:     "sdk/dir",
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

	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
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
		SnapshotDirectory:    "./testdata/snapshots",
		CoreSdkDirectory:     "sdk/dir",
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
	assert.Contains(t, failureMessage,
		"expected no error, got Error: unexpected required plugins: "+
			"actual [language-mock@<nil>], expected [language-mock@<nil> resource-simple@1.0.0]")
}
