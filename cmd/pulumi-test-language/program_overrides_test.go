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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// ProgramOverridesLanguageHost is a mock language host designed to exercise program overrides. It is only capable of
// passing the "l1-empty" conformance test (which simply registers a stack resource).
type ProgramOverridesLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string

	generateProjectCalled bool
	runCalled             bool

	GetProgramDependenciesF func(
		ctx context.Context,
		req *pulumirpc.GetProgramDependenciesRequest,
	) (*pulumirpc.GetProgramDependenciesResponse, error)

	GetRequiredPackagesF func(
		ctx context.Context,
		req *pulumirpc.GetRequiredPackagesRequest,
	) (*pulumirpc.GetRequiredPackagesResponse, error)

	GeneratePackageF func(
		ctx context.Context,
		req *pulumirpc.GeneratePackageRequest,
	) (*pulumirpc.GeneratePackageResponse, error)

	RunF func(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error)
}

func (h *ProgramOverridesLanguageHost) GeneratePackage(
	ctx context.Context,
	req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if h.GeneratePackageF != nil {
		return h.GeneratePackageF(ctx, req)
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
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
	ctx context.Context,
	req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if h.GetProgramDependenciesF != nil {
		return h.GetProgramDependenciesF(ctx, req)
	}

	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (h *ProgramOverridesLanguageHost) GetRequiredPackages(
	ctx context.Context,
	req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	if h.GetRequiredPackagesF != nil {
		return h.GetRequiredPackagesF(ctx, req)
	}

	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

func (h *ProgramOverridesLanguageHost) Run(
	ctx context.Context,
	req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	h.runCalled = true

	if h.RunF != nil {
		return h.RunF(ctx, req)
	}

	return &pulumirpc.RunResponse{}, nil
}

// Tests that a conformance test which specifies program overrides does not ask the language host to generate a project,
// but otherwise behaves as expected (validating snaphots, checking assertions, etc.).
func TestProgramOverrides_DontGenerateProgram(t *testing.T) {
	t.Parallel()

	// Arrange.
	tempDir := t.TempDir()

	ctx := t.Context()
	engine := newLanguageTestServer()

	runtime := &ProgramOverridesLanguageHost{
		tempDir: tempDir,

		RunF: func(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
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
		},
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
		ProgramOverrides: map[string]*testingrpc.PrepareLanguageTestsRequest_ProgramOverride{
			"l1-empty": {
				Paths: []string{"./tests/testdata/overrides/l1-empty"},
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

	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)

	// Assert.
	require.NoError(t, err)
	require.NotNil(t, runResponse)

	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)

	assert.False(t, runtime.generateProjectCalled, "GenerateProject should not have been called")
	assert.True(t, runtime.runCalled, "Run should have been called")
}

// Tests that a conformance test which specifies program overrides which target tests with multiple runs work correctly.
func TestProgramOverrides_WorkWithMultipleRuns(t *testing.T) {
	t.Parallel()

	// Arrange.
	tempDir := t.TempDir()

	ctx := t.Context()
	engine := newLanguageTestServer()

	runtime := &ProgramOverridesLanguageHost{
		tempDir: tempDir,

		GetProgramDependenciesF: func(
			_ context.Context,
			_ *pulumirpc.GetProgramDependenciesRequest,
		) (*pulumirpc.GetProgramDependenciesResponse, error) {
			return &pulumirpc.GetProgramDependenciesResponse{
				Dependencies: []*pulumirpc.DependencyInfo{
					{
						Name:    "simple",
						Version: "2.0.0",
					},
				},
			}, nil
		},

		GetRequiredPackagesF: func(
			_ context.Context,
			_ *pulumirpc.GetRequiredPackagesRequest,
		) (*pulumirpc.GetRequiredPackagesResponse, error) {
			return &pulumirpc.GetRequiredPackagesResponse{
				Packages: []*pulumirpc.PackageDependency{
					{
						Kind:    "resource",
						Name:    "simple",
						Version: "2.0.0",
					},
				},
			}, nil
		},

		GeneratePackageF: func(
			_ context.Context,
			req *pulumirpc.GeneratePackageRequest,
		) (*pulumirpc.GeneratePackageResponse, error) {
			if req.Directory != filepath.Join(tempDir, "sdks", "simple-2.0.0") {
				return nil, fmt.Errorf("unexpected directory %s", req.Directory)
			}

			// Write the minimal package code.
			if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
				return nil, err
			}

			return &pulumirpc.GeneratePackageResponse{}, nil
		},

		RunF: func(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
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

			if strings.HasSuffix(req.Pwd, "0") {
				_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
					Type:   "simple:index:Resource",
					Custom: true,
					Name:   "aresource",
					Object: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"value": structpb.NewBoolValue(true),
						},
					},
				})
				if err != nil {
					return nil, fmt.Errorf("could not register aresource: %w", err)
				}

				_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
					Type:   "simple:index:Resource",
					Custom: true,
					Name:   "other",
					Object: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"value": structpb.NewBoolValue(true),
						},
					},
				})
				if err != nil {
					return nil, fmt.Errorf("could not register other: %w", err)
				}

				return &pulumirpc.RunResponse{}, nil
			}

			_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
				Type:   "simple:index:Resource",
				Custom: true,
				Name:   "aresource",
				Object: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"value": structpb.NewBoolValue(true),
					},
				},
			})
			if err != nil {
				return nil, fmt.Errorf("could not register aresource: %w", err)
			}

			return &pulumirpc.RunResponse{}, nil
		},
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
		ProgramOverrides: map[string]*testingrpc.PrepareLanguageTestsRequest_ProgramOverride{
			"l2-destroy": {
				Paths: []string{
					"./tests/testdata/overrides/l2-destroy/0",
					"./tests/testdata/overrides/l2-destroy/1",
				},
			},
		},
	})

	require.NoError(t, err)
	require.NotEmpty(t, prepareResponse.Token)

	// Act.
	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "l2-destroy",
	})

	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)

	// Assert.
	require.NoError(t, err)
	require.NotNil(t, runResponse)

	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)

	assert.False(t, runtime.generateProjectCalled, "GenerateProject should not have been called")
	assert.True(t, runtime.runCalled, "Run should have been called")
}

// Tests that a conformance test which specifies program overrides which target tests with multiple runs fails if the
// number of runs does not match the number of overrides.
func TestProgramOverrides_MustMatchRuns(t *testing.T) {
	t.Parallel()

	// Arrange.
	tempDir := t.TempDir()

	ctx := t.Context()
	engine := newLanguageTestServer()

	runtime := &ProgramOverridesLanguageHost{tempDir: tempDir}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, runtime)
			return nil
		},
	})
	require.NoError(t, err)

	// Act.
	_, err = engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "mock",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   tempDir,
		SnapshotDirectory:    "./tests/testdata/snapshots",
		ProgramOverrides: map[string]*testingrpc.PrepareLanguageTestsRequest_ProgramOverride{
			// l2-destroy has 2 runs, but we only supply 1 override here.
			"l2-destroy": {
				Paths: []string{
					"./tests/testdata/overrides/l2-destroy/0",
				},
			},
		},
	})

	// Assert.
	assert.ErrorContains(t, err, "program override for test l2-destroy has 1 paths but 2 runs")
}
