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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

type L2ParameterizedResourceTwiceLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
}

func (h *L2ParameterizedResourceTwiceLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "hipackage-2.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "hipackage-2.0.0.sdk"),
		}, nil
	} else if req.PackageDirectory == filepath.Join(h.tempDir, "sdks", "byepackage-2.0.0") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "byepackage-2.0.0.sdk"),
		}, nil
	} else if req.PackageDirectory != filepath.Join(h.tempDir, "sdks", "core") {
		return &pulumirpc.PackResponse{
			ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
		}, nil
	}

	return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
}

func (h *L2ParameterizedResourceTwiceLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "l2-parameterized-resource-twice" {
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

func (h *L2ParameterizedResourceTwiceLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	// Write the minimal package code.
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func must(v []byte, err error) []byte {
	if err != nil {
		panic(fmt.Sprintf("could not decode base64: %v", err))
	}
	return v
}

var (
	hipackageParameter  = must(base64.StdEncoding.DecodeString("SGVsbG9Xb3JsZA=="))
	byepackageParameter = must(base64.StdEncoding.DecodeString("R29vZGJ5ZVdvcmxk"))
)

func (h *L2ParameterizedResourceTwiceLanguageHost) GetRequiredPackages(
	ctx context.Context, req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{
		Packages: []*pulumirpc.PackageDependency{
			{
				Name:    "parameterized",
				Kind:    string(apitype.ResourcePlugin),
				Version: "1.2.3",
				Parameterization: &pulumirpc.PackageParameterization{
					Name:    "hipackage",
					Version: "2.0.0",
					Value:   hipackageParameter,
				},
			},
			{
				Name:    "parameterized",
				Kind:    string(apitype.ResourcePlugin),
				Version: "1.2.3",
				Parameterization: &pulumirpc.PackageParameterization{
					Name:    "byepackage",
					Version: "2.0.0",
					Value:   byepackageParameter,
				},
			},
		},
	}, nil
}

func (h *L2ParameterizedResourceTwiceLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{
				Name:    "pulumi_pulumi",
				Version: "1.0.1",
			},
			{
				Name:    "hipackage",
				Version: "2.0.0",
			},
			{
				Name:    "byepackage",
				Version: "2.0.0",
			},
		},
	}, nil
}

func (h *L2ParameterizedResourceTwiceLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (h *L2ParameterizedResourceTwiceLanguageHost) Run(
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

	hipackage := promise.Run(func() (string, error) {
		hipackage, err := monitor.RegisterPackage(ctx, &pulumirpc.RegisterPackageRequest{
			Name:    "parameterized",
			Version: "1.2.3",
			Parameterization: &pulumirpc.Parameterization{
				Name:    "hipackage",
				Version: "2.0.0",
				Value:   hipackageParameter,
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not register package hipackage: %w", err)
		}
		return hipackage.Ref, nil
	})

	example1 := promise.Run(func() (*structpb.Value, error) {
		ref, err := hipackage.Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get package reference: %w", err)
		}

		example1, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:       "hipackage:index:HelloWorld",
			Custom:     true,
			Name:       "example1",
			PackageRef: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
		return example1.Object.Fields["parameterValue"], nil
	})

	exampleComponent1 := promise.Run(func() (*structpb.Value, error) {
		ref, err := hipackage.Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get package reference: %w", err)
		}

		exampleComponent1, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:       "hipackage:index:HelloWorldComponent",
			Remote:     true,
			Name:       "exampleComponent1",
			PackageRef: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
		return exampleComponent1.Object.Fields["parameterValue"], nil
	})

	byepackage := promise.Run(func() (string, error) {
		byepackage, err := monitor.RegisterPackage(ctx, &pulumirpc.RegisterPackageRequest{
			Name:    "parameterized",
			Version: "1.2.3",
			Parameterization: &pulumirpc.Parameterization{
				Name:    "byepackage",
				Version: "2.0.0",
				Value:   byepackageParameter,
			},
		})
		if err != nil {
			return "", fmt.Errorf("could not register package byepackage: %w", err)
		}
		return byepackage.Ref, nil
	})

	example2 := promise.Run(func() (*structpb.Value, error) {
		ref, err := byepackage.Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get package reference: %w", err)
		}

		example2, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:       "byepackage:index:GoodbyeWorld",
			Custom:     true,
			Name:       "example2",
			PackageRef: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
		return example2.Object.Fields["parameterValue"], nil
	})

	exampleComponent2 := promise.Run(func() (*structpb.Value, error) {
		ref, err := byepackage.Result(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get package reference: %w", err)
		}

		exampleComponent2, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:       "byepackage:index:GoodbyeWorldComponent",
			Remote:     true,
			Name:       "exampleComponent2",
			PackageRef: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("could not register resource: %w", err)
		}
		return exampleComponent2.Object.Fields["parameterValue"], nil
	})

	e1, err := example1.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get example1 result: %w", err)
	}
	ec1, err := exampleComponent1.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get exampleComponent1 result: %w", err)
	}
	e2, err := example2.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get example2 result: %w", err)
	}
	ec2, err := exampleComponent2.Result(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get exampleComponent2 result: %w", err)
	}

	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: stack.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"parameterValue1":              e1,
				"parameterValueFromComponent1": ec1,
				"parameterValue2":              e2,
				"parameterValueFromComponent2": ec2,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack outputs: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Run a simple successful test with a mocked runtime.
func TestL2ParameterizedResourceTwice(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &L2ParameterizedResourceTwiceLanguageHost{tempDir: tempDir}
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
		Test:  "l2-parameterized-resource-twice",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
