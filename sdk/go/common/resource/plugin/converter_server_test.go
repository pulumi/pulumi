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
)

type testConverter struct{}

func (c *testConverter) Close() error {
	return nil
}

func (c *testConverter) ConvertState(
	ctx context.Context, req *ConvertStateRequest,
) (*ConvertStateResponse, error) {
	if !reflect.DeepEqual(req.Args, []string{"arg1", "arg2"}) {
		return nil, fmt.Errorf("unexpected Args: %v", req.Args)
	}
	if req.MapperTarget != "localhost:1234" {
		return nil, fmt.Errorf("unexpected MapperTarget: %s", req.MapperTarget)
	}

	diags := hcl.Diagnostics{
		{
			Severity: hcl.DiagError,
			Summary:  "test:summary",
			Detail:   "test:detail",
		},
	}

	return &ConvertStateResponse{
		Resources: []ResourceImport{
			{
				Type:              "test:type",
				Name:              "test:name",
				ID:                "test:id",
				Version:           "test:version",
				PluginDownloadURL: "test:pluginDownloadURL",
				LogicalName:       "test:logicalName",
				IsRemote:          true,
				IsComponent:       true,
			},
		},
		Diagnostics: diags,
	}, nil
}

func (c *testConverter) ConvertProgram(
	ctx context.Context, req *ConvertProgramRequest,
) (*ConvertProgramResponse, error) {
	if req.MapperTarget != "localhost:1234" {
		return nil, fmt.Errorf("unexpected MapperTarget: %s", req.MapperTarget)
	}
	if req.SourceDirectory != "src" {
		return nil, fmt.Errorf("unexpected SourceDirectory: %s", req.SourceDirectory)
	}
	if req.TargetDirectory != "dst" {
		return nil, fmt.Errorf("unexpected TargetDirectory: %s", req.TargetDirectory)
	}

	diags := hcl.Diagnostics{
		{
			Severity: hcl.DiagError,
			Summary:  "test:summary",
			Detail:   "test:detail",
		},
	}

	return &ConvertProgramResponse{
		Diagnostics: diags,
	}, nil
}

func TestConverterServer_State(t *testing.T) {
	t.Parallel()

	server := NewConverterServer(&testConverter{})

	resp, err := server.ConvertState(context.Background(), &pulumirpc.ConvertStateRequest{
		Args:         []string{"arg1", "arg2"},
		MapperTarget: "localhost:1234",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Resources))

	res := resp.Resources[0]
	assert.Equal(t, "test:type", res.Type)
	assert.Equal(t, "test:name", res.Name)
	assert.Equal(t, "test:id", res.Id)
	assert.Equal(t, "test:version", res.Version)
	assert.Equal(t, "test:pluginDownloadURL", res.PluginDownloadURL)
	assert.Equal(t, "test:logicalName", res.LogicalName)
	assert.Equal(t, true, res.IsRemote)
	assert.Equal(t, true, res.IsComponent)

	diag := resp.Diagnostics[0]
	assert.Equal(t, codegenrpc.DiagnosticSeverity_DIAG_ERROR, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}

func TestConverterServer_Program(t *testing.T) {
	t.Parallel()

	server := NewConverterServer(&testConverter{})

	resp, err := server.ConvertProgram(context.Background(), &pulumirpc.ConvertProgramRequest{
		MapperTarget:    "localhost:1234",
		LoaderTarget:    "localhost:4321",
		SourceDirectory: "src",
		TargetDirectory: "dst",
		Args:            []string{"arg1", "arg2"},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, len(resp.Diagnostics))

	diag := resp.Diagnostics[0]
	assert.Equal(t, codegenrpc.DiagnosticSeverity_DIAG_ERROR, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}
