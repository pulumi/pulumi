// Copyright 2016, Pulumi Corporation.
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
				Parameterization: &ResourceParameterization{
					PluginName:    "test:pluginName",
					PluginVersion: "1.2.3",
					Value:         []byte("test:value"),
				},
				Extension: &ResourceExtension{
					Name:    "test:extension",
					Version: "4.5.6",
					Value:   []byte("test:extValue"),
				},
				Parent:     "test:parent",
				Properties: []string{"prop1", "prop2"},
				Provider:   "test:provider",
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

func (c *testConverter) ConvertSnippet(
	ctx context.Context, req *ConvertSnippetRequest,
) (*ConvertSnippetResponse, error) {
	if req.Filename != "inputs.yaml" {
		return nil, fmt.Errorf("unexpected Filename: %s", req.Filename)
	}
	if string(req.Source) != "inputs: true" {
		return nil, fmt.Errorf("unexpected Source: %s", req.Source)
	}
	if req.TargetLoader != "localhost:4321" {
		return nil, fmt.Errorf("unexpected TargetLoader: %s", req.TargetLoader)
	}
	if req.Token != "test:index:fn" {
		return nil, fmt.Errorf("unexpected Token: %s", req.Token)
	}

	diags := hcl.Diagnostics{
		{
			Severity: hcl.DiagWarning,
			Summary:  "test:summary",
			Detail:   "test:detail",
		},
	}

	return &ConvertSnippetResponse{
		Diagnostics: diags,
		Filename:    "inputs.pp",
		Source:      []byte("inputs = true"),
	}, nil
}

func TestConverterServer_State(t *testing.T) {
	t.Parallel()

	server := NewConverterServer(&testConverter{})

	resp, err := server.ConvertState(t.Context(), &pulumirpc.ConvertStateRequest{
		Args:         []string{"arg1", "arg2"},
		MapperTarget: "localhost:1234",
	})

	require.NoError(t, err)
	require.Len(t, resp.Resources, 1)

	res := resp.Resources[0]
	assert.Equal(t, "test:type", res.Type)
	assert.Equal(t, "test:name", res.Name)
	assert.Equal(t, "test:id", res.Id)
	assert.Equal(t, "test:version", res.Version)
	assert.Equal(t, "test:pluginDownloadURL", res.PluginDownloadURL)
	assert.Equal(t, "test:logicalName", res.LogicalName)
	assert.Equal(t, true, res.IsRemote)
	assert.Equal(t, true, res.IsComponent)
	require.NotNil(t, res.Parameterization)
	assert.Equal(t, "test:pluginName", res.Parameterization.PluginName)
	assert.Equal(t, "1.2.3", res.Parameterization.PluginVersion)
	assert.Equal(t, []byte("test:value"), res.Parameterization.Value)
	require.NotNil(t, res.Extension)
	assert.Equal(t, "test:extension", res.Extension.Name)
	assert.Equal(t, "4.5.6", res.Extension.Version)
	assert.Equal(t, []byte("test:extValue"), res.Extension.Value)
	assert.Equal(t, "test:parent", res.Parent)
	assert.Equal(t, []string{"prop1", "prop2"}, res.Properties)
	assert.Equal(t, "test:provider", res.Provider)

	diag := resp.Diagnostics[0]
	assert.Equal(t, codegenrpc.DiagnosticSeverity_DIAG_ERROR, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}

func TestConverterServer_ConvertSnippet(t *testing.T) {
	t.Parallel()

	server := NewConverterServer(&testConverter{})

	resp, err := server.ConvertSnippet(t.Context(), &pulumirpc.ConvertSnippetRequest{
		Filename:     "inputs.yaml",
		Source:       []byte("inputs: true"),
		TargetLoader: "localhost:4321",
		Token:        "test:index:fn",
	})

	require.NoError(t, err)
	assert.Equal(t, "inputs.pp", resp.Filename)
	assert.Equal(t, []byte("inputs = true"), resp.Source)
	require.Len(t, resp.Diagnostics, 1)

	diag := resp.Diagnostics[0]
	assert.Equal(t, codegenrpc.DiagnosticSeverity_DIAG_WARNING, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}

func TestConverterServer_Program(t *testing.T) {
	t.Parallel()

	server := NewConverterServer(&testConverter{})

	resp, err := server.ConvertProgram(t.Context(), &pulumirpc.ConvertProgramRequest{
		MapperTarget:    "localhost:1234",
		LoaderTarget:    "localhost:4321",
		SourceDirectory: "src",
		TargetDirectory: "dst",
		Args:            []string{"arg1", "arg2"},
	})

	require.NoError(t, err)
	require.Len(t, resp.Diagnostics, 1)

	diag := resp.Diagnostics[0]
	assert.Equal(t, codegenrpc.DiagnosticSeverity_DIAG_ERROR, diag.Severity)
	assert.Equal(t, "test:summary", diag.Summary)
	assert.Equal(t, "test:detail", diag.Detail)
}
