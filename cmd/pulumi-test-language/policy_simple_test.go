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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v2"
)

type PolicySimpleLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	tempDir string
}

func (h *PolicySimpleLanguageHost) Pack(
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

func (h *PolicySimpleLanguageHost) GenerateProject(
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
	if req.TargetDirectory != filepath.Join(h.tempDir, "projects", "policy-simple") {
		return nil, fmt.Errorf("unexpected target directory %s", req.TargetDirectory)
	}
	var project workspace.Project
	if err := json.Unmarshal([]byte(req.Project), &project); err != nil {
		return nil, err
	}
	if project.Name != "policy-simple" {
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

func (h *PolicySimpleLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	if req.Directory != filepath.Join(h.tempDir, "sdks", "simple-2.0.0") {
		return nil, fmt.Errorf("unexpected directory %s", req.Directory)
	}

	// Write the minimal package code.
	if err := os.WriteFile(filepath.Join(req.Directory, "test.txt"), []byte("testing"), 0o600); err != nil {
		return nil, err
	}

	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (h *PolicySimpleLanguageHost) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "policy-simple") {
		return nil, fmt.Errorf("unexpected directory to get required plugins %s", req.Info.ProgramDirectory)
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

func (h *PolicySimpleLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	if req.Info.ProgramDirectory != filepath.Join(h.tempDir, "projects", "policy-simple") {
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

func (h *PolicySimpleLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	if req.IsPlugin {
		if !strings.HasSuffix(req.Info.RootDirectory, filepath.Join("policy_packs", "simple")) {
			return fmt.Errorf("unexpected root directory to install dependencies %s", req.Info.RootDirectory)
		}
	} else {
		if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "policy-simple") {
			return fmt.Errorf("unexpected root directory to install dependencies %s", req.Info.RootDirectory)
		}
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
		return fmt.Errorf("unexpected program directory to install dependencies %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return fmt.Errorf("unexpected entry point to install dependencies %s", req.Info.EntryPoint)
	}
	return nil
}

func (h *PolicySimpleLanguageHost) Run(
	ctx context.Context, req *pulumirpc.RunRequest,
) (*pulumirpc.RunResponse, error) {
	if req.Info.RootDirectory != filepath.Join(h.tempDir, "projects", "policy-simple") {
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

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "simple:index:Resource",
		Custom: true,
		Name:   "res1",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(true),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:   "simple:index:Resource",
		Custom: true,
		Name:   "res2",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(false),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not register resource: %w", err)
	}

	return &pulumirpc.RunResponse{}, nil
}

func (h *PolicySimpleLanguageHost) Link(
	ctx context.Context, req *pulumirpc.LinkRequest,
) (*pulumirpc.LinkResponse, error) {
	if !strings.HasSuffix(req.Info.RootDirectory, filepath.Join("policy_packs", "simple")) {
		return nil, fmt.Errorf("unexpected root directory to link %s", req.Info.RootDirectory)
	}
	if req.LocalDependencies["pulumi"] != filepath.Join(h.tempDir, "artifacts", "core.sdk") {
		return nil, fmt.Errorf("unexpected core sdk %s", req.LocalDependencies["pulumi"])
	}
	return &pulumirpc.LinkResponse{}, nil
}

type analyzerPlugin struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (a *analyzerPlugin) GetAnalyzerInfo(context.Context, *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{
		Name:    "simple",
		Version: "1.0.0",
		Policies: []*pulumirpc.PolicyInfo{
			{
				Name:        "truthiness",
				Description: "Verifies properties are true",
			},
			{
				Name:        "falsiness",
				Description: "Verifies properties are false",
			},
		},
	}, nil
}

func (a *analyzerPlugin) Analyze(
	ctx context.Context, req *pulumirpc.AnalyzeRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	if req.Type == "simple:index:Resource" {
		val := req.Properties.Fields["value"]
		asBool := val.GetBoolValue()

		if asBool {
			return &pulumirpc.AnalyzeResponse{
				Diagnostics: []*pulumirpc.AnalyzeDiagnostic{
					{
						PolicyName:        "truthiness",
						PolicyPackName:    "simple",
						PolicyPackVersion: "1.0.0",
						EnforcementLevel:  pulumirpc.EnforcementLevel_ADVISORY,
						Message:           "Verifies properties are true\nThis is a test warning",
					},
				},
			}, nil
		}
		return &pulumirpc.AnalyzeResponse{
			Diagnostics: []*pulumirpc.AnalyzeDiagnostic{
				{
					PolicyName:        "falsiness",
					PolicyPackName:    "simple",
					PolicyPackVersion: "1.0.0",
					EnforcementLevel:  pulumirpc.EnforcementLevel_MANDATORY,
					Message:           "Verifies properties are false\nThis is a test error",
				},
			},
		}, nil
	}

	return &pulumirpc.AnalyzeResponse{}, nil
}

func (a *analyzerPlugin) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (h *PolicySimpleLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	if !strings.HasSuffix(req.Info.RootDirectory, filepath.Join("policy_packs", "simple")) {
		return fmt.Errorf("unexpected root directory to run plugin %s", req.Info.RootDirectory)
	}
	if req.Info.ProgramDirectory != req.Info.RootDirectory {
		return fmt.Errorf("unexpected program directory to run plugin %s", req.Info.ProgramDirectory)
	}
	if req.Info.EntryPoint != "." {
		return fmt.Errorf("unexpected entry point to run plugin %s", req.Info.EntryPoint)
	}

	// Expect this to be an analyzer plugin
	if req.Kind != string(apitype.AnalyzerPlugin) {
		return fmt.Errorf("unexpected plugin kind %s", req.Kind)
	}

	// Boot an analyzer plugin and run it
	analyzer := &analyzerPlugin{}
	stop := make(chan bool)
	go func() {
		<-server.Context().Done()
		stop <- true
	}()
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, analyzer)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return fmt.Errorf("could not start analyzer plugin: %w", err)
	}

	err = server.Send(&pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{
			Stdout: []byte(fmt.Sprintf("%v\n", handle.Port)),
		},
	})
	if err != nil {
		return fmt.Errorf("could not send plugin port: %w", err)
	}

	done := <-handle.Done
	return done
}

// Run a simple successful test with a mocked runtime.
func TestPolicySimple(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	engine := newLanguageTestServer()
	runtime := &PolicySimpleLanguageHost{tempDir: tempDir}
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
		PolicyPackDirectory:  "./tests/testdata/policies",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	runResponse, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "policy-simple",
	})
	require.NoError(t, err)
	t.Logf("stdout: %s", runResponse.Stdout)
	t.Logf("stderr: %s", runResponse.Stderr)
	assert.Empty(t, runResponse.Messages)
	assert.True(t, runResponse.Success)
}
