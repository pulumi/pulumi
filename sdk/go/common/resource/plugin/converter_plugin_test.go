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

package plugin

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type testConverterClient struct {
	diagnostics []*codegenrpc.Diagnostic
}

func (c *testConverterClient) ConvertState(
	ctx context.Context, req *pulumirpc.ConvertStateRequest, opts ...grpc.CallOption,
) (*pulumirpc.ConvertStateResponse, error) {
	if !reflect.DeepEqual(req.Args, []string{"arg1", "arg2"}) {
		return nil, fmt.Errorf("unexpected Args: %v", req.Args)
	}
	if req.MapperTarget != "localhost:1234" {
		return nil, fmt.Errorf("unexpected MapperTarget: %s", req.MapperTarget)
	}

	return &pulumirpc.ConvertStateResponse{
		Resources: []*pulumirpc.ResourceImport{
			{
				Type:              "test:type",
				Name:              "test:name",
				Id:                "test:id",
				Version:           "test:version",
				PluginDownloadURL: "test:pluginDownloadURL",
				LogicalName:       "test:logicalName",
				IsRemote:          true,
				IsComponent:       true,
			},
		},
		Diagnostics: c.diagnostics,
	}, nil
}

func (c *testConverterClient) ConvertProgram(
	ctx context.Context, req *pulumirpc.ConvertProgramRequest, opts ...grpc.CallOption,
) (*pulumirpc.ConvertProgramResponse, error) {
	if req.MapperTarget != "localhost:1234" {
		return nil, fmt.Errorf("unexpected MapperTarget: %s", req.MapperTarget)
	}
	if req.SourceDirectory != "src" {
		return nil, fmt.Errorf("unexpected SourceDirectory: %s", req.SourceDirectory)
	}
	if req.TargetDirectory != "dst" {
		return nil, fmt.Errorf("unexpected TargetDirectory: %s", req.TargetDirectory)
	}
	if !reflect.DeepEqual(req.Args, []string{"arg1", "arg2"}) {
		return nil, fmt.Errorf("unexpected args: %v", req.Args)
	}

	return &pulumirpc.ConvertProgramResponse{
		Diagnostics: c.diagnostics,
	}, nil
}

func TestConverterPlugin_State(t *testing.T) {
	t.Parallel()

	plugin := &converter{
		clientRaw: &testConverterClient{
			diagnostics: []*codegenrpc.Diagnostic{
				{
					Severity: codegenrpc.DiagnosticSeverity_DIAG_ERROR,
					Summary:  "test:summary",
					Detail:   "test:detail",
				},
			},
		},
	}

	resp, err := plugin.ConvertState(context.Background(), &ConvertStateRequest{
		Args:         []string{"arg1", "arg2"},
		MapperTarget: "localhost:1234",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Resources))

	res := resp.Resources[0]
	assert.Equal(t, "test:type", res.Type)
	assert.Equal(t, "test:name", res.Name)
	assert.Equal(t, "test:id", res.ID)
	assert.Equal(t, "test:version", res.Version)
	assert.Equal(t, "test:pluginDownloadURL", res.PluginDownloadURL)
	assert.Equal(t, "test:logicalName", res.LogicalName)
	assert.True(t, res.IsRemote)
	assert.True(t, res.IsComponent)

	diag := resp.Diagnostics[0]
	assert.Equal(t, hcl.DiagError, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}

func TestConverterPlugin_Program(t *testing.T) {
	t.Parallel()

	plugin := &converter{
		clientRaw: &testConverterClient{
			diagnostics: []*codegenrpc.Diagnostic{
				{
					Severity: codegenrpc.DiagnosticSeverity_DIAG_ERROR,
					Summary:  "test:summary",
					Detail:   "test:detail",
				},
			},
		},
	}

	resp, err := plugin.ConvertProgram(context.Background(), &ConvertProgramRequest{
		MapperTarget:    "localhost:1234",
		LoaderTarget:    "localhost:4321",
		SourceDirectory: "src",
		TargetDirectory: "dst",
		Args:            []string{"arg1", "arg2"},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Diagnostics))

	diag := resp.Diagnostics[0]
	assert.Equal(t, hcl.DiagError, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}

func TestConverterPlugin_Program_EmptyDiagnosticsIsNil(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi-terraform-bridge/issues/1201 hcl.Diagnostics
	// implements the Error interface, but this means a list of zero diagnostics still looks like a non-nil
	// error, which throws of normal "if err == nil" checks. We make sure that if the RPC diagnostics list is
	// empty we return nil not an empty slice.
	t.Parallel()

	plugin := &converter{
		clientRaw: &testConverterClient{
			diagnostics: []*codegenrpc.Diagnostic{},
		},
	}

	resp, err := plugin.ConvertProgram(context.Background(), &ConvertProgramRequest{
		MapperTarget:    "localhost:1234",
		LoaderTarget:    "localhost:4321",
		SourceDirectory: "src",
		TargetDirectory: "dst",
		Args:            []string{"arg1", "arg2"},
	})

	require.NoError(t, err)
	assert.Nil(t, resp.Diagnostics)
}
