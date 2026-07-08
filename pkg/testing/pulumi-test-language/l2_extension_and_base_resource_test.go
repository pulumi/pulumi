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

	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/runner"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/tests"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
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

type L2ExtensionAndBaseResourceLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
	tempDir string
}

func (h *L2ExtensionAndBaseResourceLanguageHost) Pack(
	_ context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "myext-2.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "myext-2.0.0.sdk"),
		}, nil
	}
	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "extbase-45.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "extbase-45.0.0.sdk"),
		}, nil
	}
	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
	}, nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) GenerateProject(
	_ context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l2-extension-and-base-resource" {
		return nil, fmt.Errorf("unexpected project name %s", project.Name)
	}
	project.Runtime = workspace.NewProjectRuntimeInfo("mock", nil)
	projectYaml, err := yaml.Marshal(project)
	if err != nil {
		return nil, fmt.Errorf("could not marshal project: %w", err)
	}
	if err := os.WriteFile(filepath.Join(req.TargetDirectory, "Pulumi.yaml"), projectYaml, 0o600); err != nil {
		return nil, err
	}
	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) GeneratePackage(
	_ context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}
	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) GetRequiredPackages(
	_ context.Context, _ *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{
		Packages: []*pulumirpc.PackageDependency{
			{
				Name:    "extbase",
				Kind:    string(apitype.ResourcePlugin),
				Version: "45.0.0",
			},
			{
				Name:    "extbase",
				Kind:    string(apitype.ResourcePlugin),
				Version: "45.0.0",
				Extension: &pulumirpc.PackageParameterization{
					Name:    "myext",
					Version: "2.0.0",
					Value:   myextParameter,
				},
			},
		},
	}, nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) GetProgramDependencies(
	_ context.Context, _ *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{Name: "pulumi_pulumi", Version: "1.0.1"},
			{Name: "extbase", Version: "45.0.0"},
			{Name: "myext", Version: "2.0.0"},
		},
	}, nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) InstallDependencies(
	_ *pulumirpc.InstallDependenciesRequest, _ pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (h *L2ExtensionAndBaseResourceLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
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
	stack, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}

	myextRef := promise.Run(func() (string, error) {
		resp, err := monitor.RegisterPackage(ctx, &pulumirpc.RegisterPackageRequest{
			Name:    "extbase",
			Version: "45.0.0",
			Extension: &pulumirpc.Parameterization{
				Name:    "myext",
				Version: "2.0.0",
				Value:   myextParameter,
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not register extension package: %w", err)
		}
		return resp.Ref, nil
	})

	// Base is the base provider's own resource: default provider, no PackageRef.
	base := promise.Run(func() (*structpb.Value, error) {
		res, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:   "extbase:index:Base",
			Custom: true,
			Name:   "base",
		})
		if err != nil {
			return nil, fmt.Errorf("could not register base: %w", err)
		}
		return res.Object.Fields["baseValue"], nil
	})

	greeting := promise.Run(func() (*structpb.Value, error) {
		ref, err := myextRef.Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get package reference: %w", err)
		}
		res, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:       "extbase:index:Greeting",
			Custom:     true,
			Name:       "greeting",
			PackageRef: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register greeting: %w", err)
		}
		return res.Object.Fields["parameterValue"], nil
	})

	g, err := greeting.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get greeting result: %w", err)
	}
	b, err := base.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get base result: %w", err)
	}

	if _, err := monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: stack.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"parameterValue": g,
				"baseValue":      b,
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("could not register stack outputs: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

func TestL2ExtensionAndBaseResource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := runner.NewLanguageTestServer(tests.LanguageTestdata, tests.LanguageTests)
	runtime := &L2ExtensionAndBaseResourceLanguageHost{tempDir: tempDir}
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
		Test:  "l2-extension-and-base-resource",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
