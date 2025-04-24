// Copyright 2016-2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

type L2ResourceAssetArchiveLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
}

func (h *L2ResourceAssetArchiveLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "asset-archive-5.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "asset-archive-5.0.0.sdk"),
		}, nil
	} else if req.PackageDirectory != filepath.Join(h.tempDir, "sdks", "core") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
		}, nil
	}

	return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
}

func (h *L2ResourceAssetArchiveLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.LocalDependencies["asset-archive"] != filepath.Join(h.tempDir, "artifacts", "asset-archive-5.0.0.sdk") {
		return nil, fmt.Errorf("unexpected asset-archive sdk %s", req.LocalDependencies["asset-archive"])
	}
	if !req.Strict {
		return nil, errors.New("expected strict to be true")
	}
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-asset-archive") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l2-resource-asset-archive" {
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

func (h *L2ResourceAssetArchiveLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.Directory != filepath.Join(h.tempDir, "sdks", "asset-archive-5.0.0") {
		return nil, fmt.Errorf("unexpected directory %s", req.Directory)
	}

	// Write the minimal package code.
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (h *L2ResourceAssetArchiveLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-asset-archive", "subdir") {
		return nil, fmt.Errorf("unexpected directory to get required plugins %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: []*pulumirpc.PluginDependency{
			{
				Name:    "asset-archive",
				Kind:    string(apitype.ResourcePlugin),
				Version: "5.0.0",
			},
		},
	}, nil
}

func (h *L2ResourceAssetArchiveLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-asset-archive", "subdir") {
		return nil, fmt.Errorf("unexpected directory to get program dependencies %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
			{
				Name:    "pulumi_asset_archive",
				Version: "5.0.0",
			},
		},
	}, nil
}

func (h *L2ResourceAssetArchiveLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-asset-archive") {
		return fmt.Errorf("unexpected root directory to install dependencies %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != filepath.Join(req.Info.RootDirectory, "subdir") {
		return fmt.Errorf("unexpected program directory to install dependencies %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return fmt.Errorf("unexpected entry point to install dependencies %s", req.Info.EntryPoint)
	}
	return nil
}

func (h *L2ResourceAssetArchiveLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-asset-archive") {
		return nil, fmt.Errorf("unexpected root directory to run %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != filepath.Join(req.Info.RootDirectory, "subdir") {
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

	// Don't calculate hashes in the program, leave it to the engine
	asset, err := plugin.MarshalAsset(&resource.Asset{
		Path: "../test.txt",
	}, plugin.MarshalOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not marshal asset: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "asset-archive:index:AssetResource",
		Custom: true,
		Name:   "ass",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": asset,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	archive, err := plugin.MarshalArchive(&resource.Archive{
		Path: "../archive.tar",
	}, plugin.MarshalOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not marshal archive: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "asset-archive:index:ArchiveResource",
		Custom: true,
		Name:   "arc",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": archive,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	folder, err := plugin.MarshalArchive(&resource.Archive{
		Path: "../folder",
	}, plugin.MarshalOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not marshal folder: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "asset-archive:index:ArchiveResource",
		Custom: true,
		Name:   "dir",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": folder,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	assarc, err := plugin.MarshalArchive(&resource.Archive{
		Assets: map[string]interface{}{
			"string": &resource.Asset{
				Text: "file contents",
			},
			"file": &resource.Asset{
				Path: "../test.txt",
			},
			"folder": &resource.Archive{
				Path: "../folder",
			},
			"archive": &resource.Archive{
				Path: "../archive.tar",
			},
		},
	}, plugin.MarshalOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not marshal asset archive: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "asset-archive:index:ArchiveResource",
		Custom: true,
		Name:   "assarc",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": assarc,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	remoteass, err := resource.NewURIAsset(
		"https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c" +
			"/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt")
	if err != nil {
		return nil, fmt.Errorf("could not create remote asset: %w", err)
	}

	mremoteass, err := plugin.MarshalAsset(remoteass, plugin.MarshalOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not marshal remote asset: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "asset-archive:index:AssetResource",
		Custom: true,
		Name:   "remoteass",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": mremoteass,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Run a simple successful test with a mocked runtime.
func TestL2ResourceAssetArchive(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ResourceAssetArchiveLanguageHost{tempDir: tempDir}
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
		Test:  "l2-resource-asset-archive",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
