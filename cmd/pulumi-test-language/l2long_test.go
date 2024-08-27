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
	"strconv"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

type L2LongLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
}

func (h *L2LongLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if req.DestinationDirectory != filepath.Join(h.tempDir, "artifacts") {
		return nil, fmt.Errorf("unexpected destination directory %s", req.DestinationDirectory)
	}

	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "long-8.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "long-8.0.0.sdk"),
		}, nil
	} else if req.PackageDirectory != filepath.Join(h.tempDir, "sdks", "core") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
		}, nil
	}

	return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
}

func (h *L2LongLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.LocalDependencies["long"] != filepath.Join(h.tempDir, "artifacts", "long-8.0.0.sdk") {
		return nil, fmt.Errorf("unexpected long sdk %s", req.LocalDependencies["long"])
	}
	if !req.Strict {
		return nil, errors.New("expected strict to be true")
	}
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-long") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l2-resource-long" {
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

func (h *L2LongLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.Directory != filepath.Join(h.tempDir, "sdks", "long-8.0.0") {
		return nil, fmt.Errorf("unexpected directory %s", req.Directory)
	}

	// Write the minimal package code.
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (h *L2LongLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-long") {
		return nil, fmt.Errorf("unexpected directory to get required plugins %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: []*pulumirpc.PluginDependency{
			{
				Name:    "long",
				Kind:    string(apitype.ResourcePlugin),
				Version: "8.0.0",
			},
		},
	}, nil
}

func (h *L2LongLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-long") {
		return nil, fmt.Errorf("unexpected directory to get program dependencies %s", req.Info.ProgramDirectory)
	}

	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
			{
				Name:    "pulumi_long",
				Version: "8.0.0",
			},
		},
	}, nil
}

func (h *L2LongLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-long") {
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

func (h *L2LongLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "l2-resource-long") {
		return nil, fmt.Errorf("unexpected root directory to run %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
		return nil, fmt.Errorf("unexpected program directory to run %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return nil, fmt.Errorf("unexpected entry point to run %s", req.Info.EntryPoint)
	}

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

	// Check if we support integers or not
	supportsIntegers, err := monitor.SupportsFeature(ctx, &pulumirpc.SupportsFeatureRequest{
		Id: "integers",
	})
	if err != nil {
		return nil, fmt.Errorf("could not check if integers are supported: %w", err)
	}

	stackResource, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}

	makeInteger := func(value string) *structpb.Value {
		if supportsIntegers.HasSupport {
			return &structpb.Value{
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							resource.SigKey: structpb.NewStringValue(resource.IntegerValueSig),
							"value":         structpb.NewStringValue(value),
						},
					},
				},
			}
		}
		float, err := strconv.ParseFloat(value, 64)
		contract.AssertNoErrorf(err, "could not parse float value %s", value)
		return structpb.NewNumberValue(float)
	}

	makeResource := func(name, value string) (*pulumirpc.RegisterResourceResponse, error) {
		res, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:   "long:index:Resource",
			Custom: true,
			Name:   name,
			Object: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"value": makeInteger(value),
				},
			},
			AcceptIntegers: true,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
		return res, nil
	}

	// Send small as a float64 to ensure it is converted to an integer in the provider.
	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "long:index:Resource",
		Custom: true,
		Name:   "small",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewNumberValue(256),
			},
		},
		AcceptIntegers: true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	// Send the rest as integers
	_, err = makeResource("min53", "-9007199254740992")
	if err != nil {
		return nil, err
	}
	_, err = makeResource("max53", "9007199254740992")
	if err != nil {
		return nil, err
	}
	_, err = makeResource("min64", "-9223372036854775808")
	if err != nil {
		return nil, err
	}
	_, err = makeResource("max64", "9223372036854775807")
	if err != nil {
		return nil, err
	}
	_, err = makeResource("uint64", "18446744073709551615")
	if err != nil {
		return nil, err
	}
	huge, err := makeResource("huge", "20000000000000000001")
	if err != nil {
		return nil, err
	}

	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: stackResource.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"result":    makeInteger("38446744073709551871"),
				"huge":      makeInteger("20000000000000000001"),
				"roundtrip": huge.Object.GetFields()["value"],
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack outputs: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Run a successful test with a mocked runtime that uses int64 values.
//
// TODO(https://github.com/pulumi/pulumi/issues/13945): enable parallel tests
//
//nolint:paralleltest // These aren't yet safe to run in parallel
func TestL2Long(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	runtime := &L2LongLanguageHost{tempDir: tempDir}
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
		CoreSdkVersion:       "1.0.1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l2-resource-long",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
