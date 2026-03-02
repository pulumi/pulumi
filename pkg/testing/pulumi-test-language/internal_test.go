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
	"gopkg.in/yaml.v2"
)

// Check that an invalid schema triggers an error
func TestInvalidSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	// We can just reuse the L1Empty host for this, it's not actually going to be used apart from prepare.
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
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	_, err = engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "internal-bad-schema",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "error loading resource type 'bad:index:Resource':")
	assert.ErrorContains(t, err, "#/resources/bad:index:Resource/properties/value/type: unknown type kind not a type")
}

// converterTestLanguageHost is a flexible mock language host for testing the converter round-trip.
// Unlike L1EmptyLanguageHost, it does not enforce hardcoded path constraints so it can serve
// both the language test pass and the subsequent converter test pass.
type converterTestLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
	tempDir string
}

func (h *converterTestLanguageHost) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	if !strings.HasSuffix(req.PackageDirectory, "/sdk/dir") {
		return nil, fmt.Errorf("unexpected package directory %s", req.PackageDirectory)
	}
	return &pulumirpc.PackResponse{
		ArtifactPath: filepath.Join(req.DestinationDirectory, "core.sdk"),
	}, nil
}

func (h *converterTestLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
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

func (h *converterTestLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{
		Dependencies: []*pulumirpc.DependencyInfo{
			{Name: "pulumi_pulumi", Version: "1.0.1"},
		},
	}, nil
}

func (h *converterTestLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (h *converterTestLanguageHost) Run(
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
	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(resource.RootStackType),
		Name: req.Stack,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register stack: %w", err)
	}
	return &pulumirpc.RunResponse{}, nil
}

// recordingConverter is a mock plugin.Converter that records calls to ConvertProgram
// and writes a minimal empty PCL program to the target directory.
type recordingConverter struct {
	called bool
}

func (r *recordingConverter) ConvertProgram(
	ctx context.Context, req *plugin.ConvertProgramRequest,
) (*plugin.ConvertProgramResponse, error) {
	r.called = true
	if err := os.MkdirAll(req.TargetDirectory, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(req.TargetDirectory, "main.pp"), []byte(""), 0o600); err != nil {
		return nil, err
	}
	return &plugin.ConvertProgramResponse{}, nil
}

func (r *recordingConverter) ConvertState(
	ctx context.Context, req *plugin.ConvertStateRequest,
) (*plugin.ConvertStateResponse, error) {
	return nil, fmt.Errorf("ConvertState not implemented")
}

func (r *recordingConverter) Close() error { return nil }

// TestConverterRoundTrip verifies that when a ConverterPluginTarget is provided,
// the language test server calls ConvertProgram and reports both LanguageTestSuccess
// and ConvertTestSuccess.
func TestConverterRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	eng := newLanguageTestServer()
	eng.DisableSnapshotWriting = true

	langHost := &converterTestLanguageHost{tempDir: tempDir}
	langHandle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, langHost)
			return nil
		},
	})
	require.NoError(t, err)

	conv := &recordingConverter{}
	convHandle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterConverterServer(srv, plugin.NewConverterServer(conv))
			return nil
		},
	})
	require.NoError(t, err)

	prepare, err := eng.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:    "mock",
		LanguagePluginTarget:  fmt.Sprintf("127.0.0.1:%d", langHandle.Port),
		TemporaryDirectory:    tempDir,
		SnapshotDirectory:     "./tests/testdata/snapshots",
		CoreSdkDirectory:      "sdk/dir",
		CoreSdkVersion:        "1.0.1",
		ConverterPluginTarget: fmt.Sprintf("127.0.0.1:%d", convHandle.Port),
	})
	require.NoError(t, err)

	result, err := eng.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepare.Token,
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", result.Stdout)
	t.Logf("stderr: %s", result.Stderr)
	assert.True(t, result.LanguageTestSuccess)
	assert.True(t, result.ConvertTestSuccess)
	assert.True(t, result.Success)
	assert.True(t, conv.called)
}
