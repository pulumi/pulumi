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

package workflow

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	pygen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockWorkflowLoader struct {
	codegenrpc.UnimplementedWorkflowLoaderServer
}

func (m *mockWorkflowLoader) GetPackageInfo(
	context.Context, *codegenrpc.GetWorkflowPackageInfoRequest,
) (*codegenrpc.GetPackageInfoResponse, error) {
	return &codegenrpc.GetPackageInfoResponse{
		Package: &codegenrpc.PackageInfo{
			Name:    "example-workflow",
			Version: "1.2.3",
		},
	}, nil
}

func (m *mockWorkflowLoader) GetGraphs(
	context.Context, *codegenrpc.GetWorkflowGraphsRequest,
) (*codegenrpc.GetGraphsResponse, error) {
	return &codegenrpc.GetGraphsResponse{
		Graphs: []*codegenrpc.GraphInfo{
			{Token: "example:index:ci"},
		},
	}, nil
}

func (m *mockWorkflowLoader) GetGraph(
	context.Context, *codegenrpc.GetWorkflowGraphRequest,
) (*codegenrpc.GetGraphResponse, error) {
	return &codegenrpc.GetGraphResponse{
		Graph: &codegenrpc.GraphInfo{Token: "example:index:ci"},
	}, nil
}

func (m *mockWorkflowLoader) GetTriggers(
	context.Context, *codegenrpc.GetWorkflowTriggersRequest,
) (*codegenrpc.GetTriggersResponse, error) {
	return &codegenrpc.GetTriggersResponse{
		Triggers: []string{"example:index:cron"},
	}, nil
}

func (m *mockWorkflowLoader) GetTrigger(
	context.Context, *codegenrpc.GetWorkflowTriggerRequest,
) (*codegenrpc.GetTriggerResponse, error) {
	return &codegenrpc.GetTriggerResponse{}, nil
}

func (m *mockWorkflowLoader) GetJobs(
	context.Context, *codegenrpc.GetWorkflowJobsRequest,
) (*codegenrpc.GetJobsResponse, error) {
	return &codegenrpc.GetJobsResponse{
		Jobs: []*codegenrpc.JobInfo{
			{Token: "example:index:build"},
		},
	}, nil
}

func (m *mockWorkflowLoader) GetJob(
	context.Context, *codegenrpc.GetWorkflowJobRequest,
) (*codegenrpc.GetJobResponse, error) {
	return &codegenrpc.GetJobResponse{
		Job: &codegenrpc.JobInfo{Token: "example:index:build"},
	}, nil
}

func TestGenerateWorkflowPackagePython(t *testing.T) {
	t.Parallel()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	codegenrpc.RegisterWorkflowLoaderServer(server, &mockWorkflowLoader{})
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	ctx := t.Context()
	conn, err := grpc.NewClient(
		"passthrough:///workflow-loader",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	outDir := t.TempDir()
	err = pygen.GenerateWorkflowPackage(
		ctx,
		outDir,
		&codegenrpc.WorkflowPackageDescriptor{
			Name:    "example",
			Version: "1.2.3",
		},
		codegenrpc.NewWorkflowLoaderClient(conn),
	)
	require.NoError(t, err)

	generated, err := os.ReadFile(filepath.Join(outDir, "__init__.py"))
	require.NoError(t, err)

	assert.Contains(t, string(generated), `PACKAGE_NAME = "example-workflow"`)
	assert.Contains(t, string(generated), `def graph_example_index_ci(`)
	assert.Contains(t, string(generated), `registry.graph("example:index:ci")(fn)`)
	assert.Contains(t, string(generated), `def trigger_example_index_cron(`)
	assert.Contains(t, string(generated), `def job_example_index_build(`)
}
