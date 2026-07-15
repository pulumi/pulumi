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

package do

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoCmdWithFunctionHelpArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:myModule:myOtherFunction": {
					Description: "This is the other function in this package.",
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
								Description: "To set param1 things",
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Type: "number",
									},
								},
								Description: "Optional values.",
							},
						},
						Required: []string{"param1"},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
								Description: "The first output.",
							},
							"output2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
								Description: "The second output.",
							},
							"output3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
								Description: "Whether it worked.",
							},
						},
						Required: []string{"output1", "output3"},
					},
				},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:myModule:myOtherFunction", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	//nolint:lll
	expected := `Invoke the myOtherFunction function.

This is the other function in this package.

Inputs:
 - param1 (string*): To set param1 things
 - param2 (Array<number>): Optional values.
Inputs marked with '*' are required

Outputs:
 - output1 (string*): The first output.
 - output2 (number): The second output.
 - output3 (boolean*): Whether it worked.
Outputs marked with '*' are always present

Usage:
  do azure:myModule:myOtherFunction [flags]

Flags:
      --dry-run                Run the operation in preview mode
  -h, --help                   help for do
      --input string           Format of the configuration files (default "yaml")
      --input-file string      Path to a file containing function inputs
      --output string          Output format for resource operation results (supported: default, json)
      --package string         The package to load, in the form 'name@version' or a path to a plugin binary or folder. If the package supports parameterization, additional space-separated parameters can be included after the package name, e.g. --package "name@version param1 \"multi word param\""
      --param1 string          To set param1 things (alias for --input:param1)
      --provider string        The URN of a provider resource in the current stack whose inputs to use as the base of the provider configuration (requires a stack context)
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output
      --stateless              Run create/patch/delete directly against the provider without persisting state. Required for now: the stateful (engine-driven) implementation is still in development, so create/patch/delete error out unless --stateless is set.
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvoke(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
							"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.False(t, req.Preview, "expected Preview to be false")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.NewProperty(false),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": false
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeFiltersOutputsToSchema(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"result": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result":     resource.NewProperty("visible"),
							"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
							"extra":      resource.NewProperty("hidden"),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "result": "visible"
}
`
	assert.Equal(t, expected, stdout.String())
}

// TestDoCmdFunctionInvokeFiltersNestedObjectsInCollections asserts that filterOutput recurses through array and map
// types: objects inside an array<object> or map<object> output should have their non-schema properties (e.g.
// __defaults, extra) stripped, the same way they are at the top level.
func TestDoCmdFunctionInvokeFiltersNestedObjectsInCollections(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Types: map[string]schema.ComplexTypeSpec{
				"azure:index:Item": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"items": {
								TypeSpec: schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Ref: "#/types/azure:index:Item",
									},
								},
							},
							"itemsByKey": {
								TypeSpec: schema.TypeSpec{
									Type: "object",
									AdditionalProperties: &schema.TypeSpec{
										Ref: "#/types/azure:index:Item",
									},
								},
							},
						},
					},
				},
			},
		}
		extras := resource.PropertyMap{
			"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
			"extra":      resource.NewProperty("hidden"),
		}
		listItem := resource.PropertyMap{
			"name":       resource.NewProperty("list-item"),
			"__defaults": extras["__defaults"],
			"extra":      extras["extra"],
		}
		mapItem := resource.PropertyMap{
			"name":       resource.NewProperty("map-item"),
			"__defaults": extras["__defaults"],
			"extra":      extras["extra"],
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"items": resource.NewProperty([]resource.PropertyValue{resource.NewProperty(listItem)}),
							"itemsByKey": resource.NewProperty(resource.PropertyMap{
								"a": resource.NewProperty(mapItem),
							}),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "items": [
    {
      "name": "list-item"
    }
  ],
  "itemsByKey": {
    "a": {
      "name": "map-item"
    }
  }
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeReturnType(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					ReturnType: &schema.ReturnTypeSpec{
						TypeSpec: &schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("visible"),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `"visible"
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeReturnTypeFiltersSchema(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Types: map[string]schema.ComplexTypeSpec{
				"azure:index:MyFunctionResult": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"details": {
								TypeSpec: schema.TypeSpec{
									Ref: "#/types/azure:index:MyFunctionDetails",
								},
							},
						},
					},
				},
				"azure:index:MyFunctionDetails": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"enabled": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					ReturnType: &schema.ReturnTypeSpec{
						TypeSpec: &schema.TypeSpec{
							Ref: "#/types/azure:index:MyFunctionResult",
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"payload": resource.NewProperty(resource.PropertyMap{
								"name": resource.NewProperty("visible"),
								"details": resource.NewProperty(resource.PropertyMap{
									"enabled":    resource.NewProperty(true),
									"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
									"extra":      resource.NewProperty("hidden"),
								}),
								"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
								"extra":      resource.NewProperty("hidden"),
							}),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "details": {
    "enabled": true
  },
  "name": "visible"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeReturnTypeFiltersSchemaSecrets(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Types: map[string]schema.ComplexTypeSpec{
				"azure:index:MyFunctionResult": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"details": {
								TypeSpec: schema.TypeSpec{
									Ref: "#/types/azure:index:MyFunctionDetails",
								},
							},
						},
					},
				},
				"azure:index:MyFunctionDetails": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"enabled": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					ReturnType: &schema.ReturnTypeSpec{
						TypeSpec: &schema.TypeSpec{
							Ref: "#/types/azure:index:MyFunctionResult",
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"payload": resource.NewProperty(resource.PropertyMap{
								"name": resource.NewProperty("visible"),
								"details": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
									"enabled":    resource.NewProperty(true),
									"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
									"extra":      resource.NewProperty("hidden"),
								})),
								"__defaults": resource.NewProperty([]resource.PropertyValue{resource.NewProperty("internal")}),
								"extra":      resource.NewProperty("hidden"),
							}),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "details": "[secret]",
  "name": "visible"
}
`
	assert.Equal(t, expected, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"azure:index:myFunction", "--show-secrets"})
	err = cmd.Execute()
	require.NoError(t, err)

	expected = `{
  "details": {
    "enabled": true
  },
  "name": "visible"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeNestedModule(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "pkg", source)
		spec := schema.PackageSpec{
			Name: "pkg",
			Functions: map[string]schema.FunctionSpec{
				"pkg:mod1/mod2:fun": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param": {
								TypeSpec: schema.TypeSpec{Type: "string"},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"result": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "pkg:mod1/mod2:fun", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("world"),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param = "hello"
`)

	stdout.Reset()
	cmd.SetArgs([]string{"pkg:mod1/mod2:fun", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "result": "world"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvoke_MissingRequiredInput(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
						Required: []string{"param1"},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
							"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.ErrorContains(t, err, `Missing required input "param1"`)
}

// TestDoCmdFunctionInvoke_NoInputFileWithRequired asserts that invoking a function which declares required inputs but
// without --input-file is rejected before the provider is called.
func TestDoCmdFunctionInvoke_NoInputFileWithRequired(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
						Required: []string{"param1"},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "provider invoke should not be called when required inputs are missing")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.ErrorContains(t, err, `Missing required input "param1"`)
}

func TestDoCmdFunctionInvokeInputFileSchemaValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantErrs []string
	}{
		{
			name: "extra property",
			input: `
param1 = "hello"
extra = true
`,
			wantErrs: []string{"unsupported attribute 'extra'"},
		},
		{
			name: "list for scalar",
			input: `
param1 = ["hello"]
`,
			wantErrs: []string{
				`Cannot assign value to input "param1"`,
				`to input "param1" of type string`,
				`Cannot assign value (string)`,
			},
		},
		{
			name: "missing required property",
			input: `
param2 = 42
`,
			wantErrs: []string{`Missing required input "param1"`},
		},
		{
			name: "scalar for list",
			input: `
param1 = "hello"
param4 = "tag"
`,
			wantErrs: []string{
				`Cannot assign value to input "param4"`,
				`Cannot assign value string to input "param4" of type list(string)`,
			},
		},
		{
			name: "wrong list element type",
			input: `
param1 = "hello"
param4 = ["tag", []]
`,
			wantErrs: []string{
				`Cannot assign value to input "param4"`,
				`Cannot assign value (string, ()) to input "param4" of type list(string)`,
			},
		},
		{
			name: "object for boolean",
			input: `
param1 = "hello"
param3 = {
    enabled = true
}
`,
			wantErrs: []string{
				`Cannot assign value to input "param3"`,
				`Cannot assign value { enabled: true } to input "param3" of type bool`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mlm := &cmdBackend.MockLoginManager{}
			mws := &pkgWorkspace.MockContext{}
			loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
				assert.Equal(t, "azure", source)
				spec := schema.PackageSpec{
					Name: "azure",
					Functions: map[string]schema.FunctionSpec{
						"azure:index:myFunction": {
							Inputs: &schema.ObjectTypeSpec{
								Properties: map[string]schema.PropertySpec{
									"param1": {TypeSpec: schema.TypeSpec{Type: "string"}},
									"param2": {TypeSpec: schema.TypeSpec{Type: "number"}},
									"param3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
									"param4": {
										TypeSpec: schema.TypeSpec{
											Type: "array",
											Items: &schema.TypeSpec{
												Type: "string",
											},
										},
									},
								},
								Required: []string{"param1"},
							},
							Outputs: &schema.ObjectTypeSpec{
								Properties: map[string]schema.PropertySpec{
									"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
								},
							},
						},
					},
				}
				return &testProvider{
					spec: spec,
					MockProvider: plugin.MockProvider{
						InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
							require.Fail(t, "provider invoke should not be called with invalid inputs")
							return plugin.InvokeResponse{}, nil
						},
					},
				}, nil
			}

			var stdout bytes.Buffer
			cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)

			inputFile := writeHCLFile(t, "inputs.pcl", tt.input)
			cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
			err := cmd.Execute()
			require.Error(t, err)
			for _, want := range tt.wantErrs {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

// TestDoCmdFunctionInvokeInputFileForInputlessFunction asserts that attributes supplied in --input-file are rejected
// at typecheck time when the target function declares no inputs. Without this, extra attributes pass through to the
// provider's Invoke call instead of producing a clear "unsupported attribute" diagnostic.
func TestDoCmdFunctionInvokeInputFileForInputlessFunction(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				// Note: no Inputs field — this function takes no arguments.
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"ok": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "provider invoke should not be called when input attributes are unsupported")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.pcl", `bogus = "hello"`)

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported attribute 'bogus'")
}

// TestDoCmdFunctionInvokeInputFileRejectsHCLBlocks asserts that PCL input files containing top-level HCL blocks
// (e.g. `something { ... }`) are rejected at bind time. Without this, blocks would be silently dropped — easy to
// mistake for "the schema doesn't honor my settings" rather than a syntax mistake.
func TestDoCmdFunctionInvokeInputFileRejectsHCLBlocks(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "provider invoke should not be called when input file contains HCL blocks")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	// PCL syntax for nested objects is `param = { ... }` — using HCL block syntax (`stuff { ... }`) is a
	// frequent mistake and should be flagged rather than silently dropped.
	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
stuff {
    nested = "value"
}
`)

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, `unexpected block "stuff"`)
}

func TestDoCmdFunctionInvokeInputFileSchemaConversions(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"param2": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
							"param3": {TypeSpec: schema.TypeSpec{Type: "number"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "44", req.Args["param1"].StringValue())
					assert.True(t, req.Args["param2"].BoolValue())
					assert.Equal(t, 45.0, req.Args["param3"].NumberValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = 44
param2 = "true"
param3 = "45"
`)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeDryRun(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
							"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Truef(t, req.Preview, "expected Preview to be true")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.MakeComputed(resource.NewProperty("")),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"--dry-run", "azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": "[unknown]"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeWithBuiltinFunctions(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"ok": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, `{"value":true}`, req.Args["param1"].StringValue())
					assert.Equal(t, 6.0, req.Args["param2"].NumberValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"ok": resource.NewProperty(true),
						},
					}, nil
				},
			},
		}, nil
	}

	dataFile := writeHCLFile(t, "data.txt", `{"value":true}`)
	inputFile := writeHCLFile(t, "inputs.pcl", fmt.Sprintf(`
param1 = readFile(%q)
param2 = max(1, length(split(":", "a:b:c")), 6)
`, dataFile))

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDoCmdFunctionInvokeWithUnsupportedBuiltinFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "project",
			input:    `param1 = project()`,
			expected: "project is not supported",
		},
		{
			name:     "rootDirectory",
			input:    `param1 = rootDirectory()`,
			expected: "rootDirectory is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mlm := &cmdBackend.MockLoginManager{}
			mws := &pkgWorkspace.MockContext{}
			loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
				spec := schema.PackageSpec{
					Name: "azure",
					Functions: map[string]schema.FunctionSpec{
						"azure:index:myFunction": {
							Inputs: &schema.ObjectTypeSpec{
								Properties: map[string]schema.PropertySpec{
									"param1": {
										TypeSpec: schema.TypeSpec{
											Type: "string",
										},
									},
								},
							},
							Outputs: &schema.ObjectTypeSpec{
								Properties: map[string]schema.PropertySpec{
									"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
									"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
									"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
								},
							},
						},
					},
				}
				return &testProvider{spec: spec}, nil
			}

			var stdout bytes.Buffer
			cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)

			inputFile := writeHCLFile(t, "inputs.pcl", tt.input)

			cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
			err := cmd.Execute()
			require.ErrorContains(t, err, tt.expected)
		})
	}
}

func TestDoCmdFunctionInvokeWithProjectContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mainDir := filepath.Join(root, "infra")
	require.NoError(t, os.Mkdir(mainDir, 0o700))

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func(_ string) (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("my-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
				Main:    "infra",
			}, root, nil
		},
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, mainDir, wd)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"pwd": {
								TypeSpec: schema.TypeSpec{Type: "string"},
							},
							"root": {
								TypeSpec: schema.TypeSpec{Type: "string"},
							},
							"project": {
								TypeSpec: schema.TypeSpec{Type: "string"},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"ok": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, mainDir, req.Args["pwd"].StringValue())
					assert.Equal(t, root, req.Args["root"].StringValue())
					assert.Equal(t, "my-project", req.Args["project"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"ok": resource.NewProperty(true),
						},
					}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.pcl", `
pwd = cwd()
root = rootDirectory()
project = project()
`)

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "pcl", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDoCmdFunctionInvokeWithConfiguration(t *testing.T) {
	t.Parallel()

	configureCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Provider: &schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"opt1": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
							"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ConfigureF: func(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					configureCalled = true
					assert.Equal(t, "val1", req.Inputs["opt1"].StringValue())
					return plugin.ConfigureResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.False(t, req.Preview, "expected Preview to be false")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.NewProperty(false),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	providerFile := writeHCLFile(t, "provider.pcl", `
opt1 = "val1"
`)
	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{
		"azure:index:myFunction",
		"--input", "pcl",
		"--provider-file", providerFile,
		"--input-file", inputFile,
	})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, configureCalled, "expected Configure to be called")

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": false
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeNestedResults(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"secret": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"list": {
								TypeSpec: schema.TypeSpec{
									Type:  "array",
									Items: &schema.TypeSpec{Type: "string"},
								},
							},
							"object": {
								TypeSpec: schema.TypeSpec{
									Type: "object",
									AdditionalProperties: &schema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.False(t, req.Preview, "expected Preview to be false")
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Empty(t, req.Args, "expected Args to be empty")
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"secret": resource.MakeSecret(resource.NewProperty("hello")),
							"list": resource.NewProperty([]resource.PropertyValue{
								resource.NewProperty("a"),
								resource.NewProperty("b"),
							}),
							"object": resource.NewProperty(resource.PropertyMap{
								"nested": resource.NewProperty("value"),
							}),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "list": [
    "a",
    "b"
  ],
  "object": {
    "nested": "value"
  },
  "secret": "[secret]"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeShowSecrets(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"secret": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"object": {
								TypeSpec: schema.TypeSpec{
									Type: "object",
									AdditionalProperties: &schema.TypeSpec{
										Type: "string",
									},
								},
							},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"secret": resource.MakeSecret(resource.NewProperty("hello")),
							"object": resource.NewProperty(resource.PropertyMap{
								"nested": resource.MakeSecret(resource.NewProperty("value")),
							}),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"--show-secrets", "azure:index:myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "object": {
    "nested": "value"
  },
  "secret": "hello"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeAssetArchiveResults(t *testing.T) {
	t.Parallel()

	textAsset, err := asset.FromText("hello from an asset")
	require.NoError(t, err)
	literalArchive, err := archive.FromAssets(map[string]any{
		"file.txt": textAsset,
	})
	require.NoError(t, err)

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"asset": {
								TypeSpec: schema.TypeSpec{
									Ref: "pulumi.json#/Asset",
								},
							},
							"archive": {
								TypeSpec: schema.TypeSpec{
									Ref: "pulumi.json#/Archive",
								},
							},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"asset":   resource.NewProperty(textAsset),
							"archive": resource.NewProperty(literalArchive),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction"})
	err = cmd.Execute()
	require.NoError(t, err)

	expected := fmt.Sprintf(`{
  "archive": {
    "4dabf18193072939515e22adb298388d": "0def7320c3a5731c473e5ecbe6d01bc7",
    "assets": {
      "file.txt": {
        "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
        "hash": %q,
        "text": "hello from an asset"
      }
    },
    "hash": %q
  },
  "asset": {
    "4dabf18193072939515e22adb298388d": "c44067f5952c0a294b673a41bacd8c17",
    "hash": %q,
    "text": "hello from an asset"
  }
}
`, textAsset.Hash, literalArchive.Hash, textAsset.Hash)
	assert.Equal(t, expected, stdout.String())
}

// TestDoCmdFunctionInvokeWithParameterizedPackage exercises the parameterized-provider path: when the user quotes
// `"<base-provider> <param1> <param2> ..."` as the first argument, `do` shlex-splits it, loads the base provider,
// and calls Parameterize with the remaining tokens before fetching the schema. The schema used for the function
// tree and the Invoke call is then for the parameterized subpackage.
func TestDoCmdFunctionInvokeWithParameterizedPackage(t *testing.T) {
	t.Parallel()

	parameterizeCalled := false
	getSchemaCalled := false
	subpackageVersion := semver.MustParse("1.2.3")

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		// shlex-split takes only the first token as the plugin source; the rest go to Parameterize.
		assert.Equal(t, "terraform-provider", source)
		spec := schema.PackageSpec{
			Name: "myparam",
			Functions: map[string]schema.FunctionSpec{
				"myparam:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ParameterizeF: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
					parameterizeCalled = true
					args, ok := req.Parameters.(*plugin.ParameterizeArgs)
					require.True(t, ok, "expected ParameterizeArgs, got %T", req.Parameters)
					assert.Equal(t, []string{"foo/bar", "1.2.3"}, args.Args)
					return plugin.ParameterizeResponse{
						Name:    "myparam",
						Version: subpackageVersion,
					}, nil
				},
				GetSchemaF: func(ctx context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					getSchemaCalled = true
					// The schema request after Parameterize should target the subpackage by name and version.
					assert.Equal(t, "myparam", req.SubpackageName)
					require.NotNil(t, req.SubpackageVersion)
					assert.Equal(t, subpackageVersion.String(), req.SubpackageVersion.String())
					schemaBytes, err := json.Marshal(schema.PackageSpec{
						Name: "myparam",
						Functions: map[string]schema.FunctionSpec{
							"myparam:index:myFunction": {
								Inputs: &schema.ObjectTypeSpec{
									Properties: map[string]schema.PropertySpec{
										"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
									},
								},
								Outputs: &schema.ObjectTypeSpec{
									Properties: map[string]schema.PropertySpec{
										"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
									},
								},
							},
						},
					})
					require.NoError(t, err)
					return plugin.GetSchemaResponse{Schema: schemaBytes}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "myparam:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["x"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"y": resource.NewProperty("world"),
						},
					}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.pcl", `x = "hello"`)

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	// First positional is the package spec: base provider name plus any Parameterize args, shlex-quoted.
	cmd.SetArgs([]string{
		"--package", "terraform-provider foo/bar 1.2.3",
		"myparam:index:myFunction", "--input", "pcl", "--input-file", inputFile,
	})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.True(t, parameterizeCalled, "expected Parameterize to be called")
	assert.True(t, getSchemaCalled, "expected GetSchema to be called")

	expected := `{
  "y": "world"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeWithYAMLInputFile(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		// Serve the standard schema loader so the context exposes a non-empty LoaderAddr, which
		// `do` forwards to the converter as its TargetLoader.
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	loadConverter := func(
		pctx *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		require.NotNil(t, pctx)
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				assert.Equal(t, "inputs.yaml", filepath.Base(req.Filename))
				assert.Equal(t, `
param1: hello
param2: 42
param3: true
`, string(req.Source))
				assert.NotEmpty(t, req.TargetLoader)
				assert.Equal(t, "azure:index:myFunction", req.Token)
				// The package descriptor we hand to the converter should match the package the user typed.
				// No version was specified and no parameterization was used.
				require.NotNil(t, req.Package)
				assert.Equal(t, "azure", req.Package.Package)
				assert.Empty(t, req.Package.Version)
				assert.Nil(t, req.Package.Parameterization)
				return &plugin.ConvertSnippetResponse{
					Filename: "inputs.pp",
					Source: []byte(`
param1 = "hello"
param2 = 42
param3 = true
`),
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"param1": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"param2": {
								TypeSpec: schema.TypeSpec{
									Type: "number",
								},
							},
							"param3": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"output2": {TypeSpec: schema.TypeSpec{Type: "number"}},
							"output3": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "azure:index:myFunction", string(req.Tok))
					assert.Equal(t, "hello", req.Args["param1"].StringValue())
					assert.Equal(t, 42.0, req.Args["param2"].NumberValue())
					assert.Equal(t, true, req.Args["param3"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output1": resource.NewProperty("world"),
							"output2": resource.NewProperty(43.0),
							"output3": resource.NewProperty(false),
						},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.yaml", `
param1: hello
param2: 42
param3: true
`)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "yaml", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": false
}
`
	assert.Equal(t, expected, stdout.String())
}

// TestDoCmdFunctionInvokeYAMLInputByDefault verifies that YAML is the default input format: an input file
// passed without --input is routed through the yaml converter.
func TestDoCmdFunctionInvokeYAMLInputByDefault(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		// Serve the standard schema loader so the context exposes a non-empty LoaderAddr, which
		// `do` forwards to the converter as its TargetLoader.
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				assert.Equal(t, "inputs.yaml", filepath.Base(req.Filename))
				assert.Equal(t, "x: hello\n", string(req.Source))
				return &plugin.ConvertSnippetResponse{
					Filename: "inputs.pp",
					Source:   []byte(`x = "hello"` + "\n"),
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "hello", req.Args["x"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"y": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.yaml", "x: hello\n")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "y": "world"
}
`
	assert.Equal(t, expected, stdout.String())
}

// TestDoCmdFunctionInvokeWithYAMLInputFileParameterized exercises the ConvertSnippet Package descriptor for a
// parameterized package — version comes from the @version suffix and Parameterization is populated from the
// provider's Parameterize response.
func TestDoCmdFunctionInvokeWithYAMLInputFileParameterized(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		// Serve the standard schema loader so the context exposes a non-empty LoaderAddr, which
		// `do` forwards to the converter as its TargetLoader.
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	subVersion := semver.MustParse("1.2.3")
	parameterValue := []byte("opaque-parameter-blob")
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				// Package descriptor should carry the base provider name and version split out from the
				// "terraform-provider@2.0.0" spec, plus the parameterization info we got back from Parameterize.
				// The Value field is sourced from the schema's own Parameterization.Parameter bytes so the
				// converter can call the loader and get back the same parameterized schema we did.
				require.NotNil(t, req.Package)
				assert.Equal(t, "terraform-provider", req.Package.Package)
				assert.Equal(t, "2.0.0", req.Package.Version)
				require.NotNil(t, req.Package.Parameterization)
				assert.Equal(t, "myparam", req.Package.Parameterization.Name)
				assert.Equal(t, subVersion.String(), req.Package.Parameterization.Version)
				assert.Equal(t, parameterValue, req.Package.Parameterization.Value)
				return &plugin.ConvertSnippetResponse{
					Filename: "inputs.pp",
					Source:   []byte(`x = "hello"` + "\n"),
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "terraform-provider@2.0.0", source)
		spec := schema.PackageSpec{
			Name: "myparam",
			Parameterization: &schema.ParameterizationSpec{
				BaseProvider: schema.BaseProviderSpec{Name: "terraform-provider", Version: "2.0.0"},
				Parameter:    parameterValue,
			},
			Functions: map[string]schema.FunctionSpec{
				"myparam:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ParameterizeF: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
					args, ok := req.Parameters.(*plugin.ParameterizeArgs)
					require.True(t, ok)
					assert.Equal(t, []string{"foo/bar", "1.2.3"}, args.Args)
					return plugin.ParameterizeResponse{Name: "myparam", Version: subVersion}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "hello", req.Args["x"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"y": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.yaml", `x: hello`+"\n")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--package", "terraform-provider@2.0.0 foo/bar 1.2.3",
		"myparam:index:myFunction", "--input", "yaml", "--input-file", inputFile,
	})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestDoCmdFunctionInvokeParameterizedSchemaWithoutArgs asserts the error path where a provider returns a
// parameterized schema but the user invoked `pulumi do` without any parameterization args. We can't faithfully
// describe such a package to a downstream converter, so the CLI surfaces the mismatch instead of silently
// constructing a half-formed descriptor.
func TestDoCmdFunctionInvokeParameterizedSchemaWithoutArgs(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Parameterization: &schema.ParameterizationSpec{
				BaseProvider: schema.BaseProviderSpec{Name: "azure", Version: "1.0.0"},
				Parameter:    []byte("opaque-parameter-blob"),
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ParameterizeF: func(ctx context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
					require.Fail(t, "Parameterize should not be called when no args were supplied")
					return plugin.ParameterizeResponse{}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "provider returned parameterization but no parameterization args were sent")
}

// TestDoCmdFunctionInvokeWithYAMLProviderFile exercises the provider-config converter path: --input yaml
// + --provider-file p.yaml should run the YAML through the converter, hand the resulting PCL to Configure, and
// pass the right token (the provider's pulumi:providers:<pkg> token) and the same package descriptor we use for
// function inputs.
func TestDoCmdFunctionInvokeWithYAMLProviderFile(t *testing.T) {
	t.Parallel()

	configureCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		// Serve the standard schema loader so the context exposes a non-empty LoaderAddr, which
		// `do` forwards to the converter as its TargetLoader.
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				assert.Equal(t, "provider.yaml", filepath.Base(req.Filename))
				assert.Equal(t, "opt1: val1\n", string(req.Source))
				assert.NotEmpty(t, req.TargetLoader)
				// The converter should be told this is a provider-config snippet via the provider's resource token,
				// not the function token.
				assert.Equal(t, "pulumi:providers:azure", req.Token)
				require.NotNil(t, req.Package)
				assert.Equal(t, "azure", req.Package.Package)
				return &plugin.ConvertSnippetResponse{
					Filename: "provider.pp",
					Source:   []byte(`opt1 = "val1"` + "\n"),
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Provider: &schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"opt1": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ConfigureF: func(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					configureCalled = true
					// The converted PCL ("opt1 = \"val1\"") should be bound, evaluated, and reach Configure intact.
					assert.Equal(t, "val1", req.Inputs["opt1"].StringValue())
					return plugin.ConfigureResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"output1": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	providerFile := writeHCLFile(t, "provider.yaml", "opt1: val1\n")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"azure:index:myFunction",
		"--provider-file", providerFile, "--input", "yaml",
	})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, configureCalled, "Configure should be called with the converted provider config")
}

// TestDoCmdFunctionInvokeWithUnknownInputFormat verifies that an unknown --input format surfaces as an error from
// the converter loader rather than crashing or silently falling through.
func TestDoCmdFunctionInvokeWithUnknownInputFormat(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	host := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		return &plugin.MockHost{}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "fictional", name)
		return nil, errors.New("converter not found")
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "Invoke should not be called when the converter fails to load")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.fictional", "x: hello")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, host, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "fictional", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "load fictional input converter")
	assert.ErrorContains(t, err, "converter not found")
}

// TestDoCmdFunctionInvokeWithConverterMissingConvertSnippet verifies that a converter plugin that exists but does not
// implement ConvertSnippet produces a targeted error instead of leaking the raw RPC error.
func TestDoCmdFunctionInvokeWithConverterMissingConvertSnippet(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	host := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		return &plugin.MockHost{}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "Invoke should not be called when the converter cannot convert snippets")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.yaml", "x: hello")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, host, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:myFunction", "--input", "yaml", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "yaml input converter does not support snippet conversion; "+
		"use pcl format or try installing a newer yaml converter")
	assert.NotContains(t, err.Error(), "ConvertSnippet not implemented")
}

// TestDoCmdFunctionInvokeWithConverterDiagnostics asserts that diagnostic-level errors from ConvertSnippet are
// surfaced and that Invoke is never called.
func TestDoCmdFunctionInvokeWithConverterDiagnostics(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	host := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		return &plugin.MockHost{}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				return &plugin.ConvertSnippetResponse{
					Diagnostics: hcl.Diagnostics{
						{Severity: hcl.DiagError, Summary: "could not convert: synthetic failure"},
					},
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "Invoke should not be called when ConvertSnippet returns error diagnostics")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.yaml", "x: hello")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, host, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "yaml", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "could not convert: synthetic failure")
}

// TestDoCmdFunctionInvokeWithConverterReturningInvalidPCL asserts that valid-looking PCL from the converter still
// has to satisfy the function's schema — extra attributes that aren't part of the function's inputs are caught
// by the bind step.
func TestDoCmdFunctionInvokeWithConverterReturningInvalidPCL(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	host := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		return &plugin.MockHost{}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				// PCL parses fine but `not_an_input` isn't part of the function's schema — the bind step should reject it.
				return &plugin.ConvertSnippetResponse{
					Filename: "inputs.pp",
					Source:   []byte(`x = "hello"` + "\n" + `not_an_input = true` + "\n"),
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					require.Fail(t, "Invoke should not be called when the converted PCL fails to bind")
					return plugin.InvokeResponse{}, nil
				},
			},
		}, nil
	}

	inputFile := writeHCLFile(t, "inputs.yaml", "x: hello")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, host, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure:index:myFunction", "--input", "yaml", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "not_an_input")
}

func TestDoCmdFunctionInvokeWithFlags(t *testing.T) {
	t.Parallel()

	configureCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Provider: &schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"opt1":   {TypeSpec: schema.TypeSpec{Type: "string"}},
					"optTwo": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"in1":    {TypeSpec: schema.TypeSpec{Type: "string"}},
							"inTwo":  {TypeSpec: schema.TypeSpec{Type: "string"}},
							"dryRun": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ConfigureF: func(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					configureCalled = true
					// The converted PCL ("opt1 = \"val1\"") + optTwo=val2 should be bound, evaluated, and reach Configure intact.
					assert.Equal(t, "val1", req.Inputs["opt1"].StringValue())
					assert.Equal(t, "val2", req.Inputs["optTwo"].StringValue())
					return plugin.ConfigureResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "p1", req.Args["in1"].StringValue())
					assert.Equal(t, "p2", req.Args["inTwo"].StringValue())
					assert.Equal(t, true, req.Args["dryRun"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"output1": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	providerFile := writeHCLFile(t, "provider.pcl", "opt1 = \"val1\"\n")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"azure:index:myFunction",
		"--input", "pcl",
		"--provider-file", providerFile,
		"--azure:opt-two", "val2",
		"--in1", "p1",
		"--input:in-two", "p2",
		"--input:dry-run",
	})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, configureCalled, "Configure should be called with the converted provider config")
}

func TestDoCmdFunctionInvokeWithYAMLFlags(t *testing.T) {
	t.Parallel()

	configureCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		// Serve the standard schema loader so the context exposes a non-empty LoaderAddr, which
		// `do` forwards to the converter as its TargetLoader.
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				assert.NotEmpty(t, req.TargetLoader)
				switch filepath.Base(req.Filename) {
				case "provider.yaml":
					assert.Equal(t, "opt1: val1\n", string(req.Source))
					assert.Equal(t, map[string]string{
						"optTwo": "val2",
					}, req.Attributes)
					// The converter should be told this is a provider-config snippet via the provider's resource token,
					// not the function token.
					assert.Equal(t, "pulumi:providers:azure", req.Token)
					require.NotNil(t, req.Package)
					assert.Equal(t, "azure", req.Package.Package)
					return &plugin.ConvertSnippetResponse{
						Filename: "provider.pp",
						Source:   []byte(`opt1 = "val1"` + "\n"),
						Attributes: map[string]string{
							"optTwo": "\"val2\"",
						},
					}, nil
				case "inputs.yaml":
					assert.Equal(t, "in1: file\n", string(req.Source))
					assert.Equal(t, map[string]string{
						"dryRun": "true",
						"in1":    "p1",
						"inTwo":  "p2",
					}, req.Attributes)
					assert.Equal(t, "azure:index:myFunction", req.Token)
					require.NotNil(t, req.Package)
					assert.Equal(t, "azure", req.Package.Package)
					return &plugin.ConvertSnippetResponse{
						Filename: "inputs.pp",
						Source:   []byte(`in1 = "file"` + "\n"),
						Attributes: map[string]string{
							"in1":    "\"p1\"",
							"inTwo":  "\"p2\"",
							"dryRun": "true",
						},
					}, nil
				default:
					require.Failf(t, "unexpected converter input", "filename: %s", req.Filename)
					return nil, nil
				}
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Provider: &schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"opt1":   {TypeSpec: schema.TypeSpec{Type: "string"}},
					"optTwo": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"in1":    {TypeSpec: schema.TypeSpec{Type: "string"}},
							"inTwo":  {TypeSpec: schema.TypeSpec{Type: "string"}},
							"dryRun": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
						},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				ConfigureF: func(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					configureCalled = true
					// The converted PCL ("opt1 = \"val1\"") + optTwo=val2 should be bound, evaluated, and reach Configure intact.
					assert.Equal(t, "val1", req.Inputs["opt1"].StringValue())
					assert.Equal(t, "val2", req.Inputs["optTwo"].StringValue())
					return plugin.ConfigureResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "p1", req.Args["in1"].StringValue())
					assert.Equal(t, "p2", req.Args["inTwo"].StringValue())
					assert.Equal(t, true, req.Args["dryRun"].BoolValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"output1": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	providerFile := writeHCLFile(t, "provider.yaml", "opt1: val1\n")
	inputFile := writeHCLFile(t, "inputs.yaml", "in1: file\n")

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"azure:index:myFunction",
		"--provider-file", providerFile,
		"--input-file", inputFile, "--input", "yaml",
		"--azure:opt-two", "val2",
		"--in1", "p1",
		"--input:in-two", "p2",
		"--input:dry-run",
	})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, configureCalled, "Configure should be called with the converted provider config")
}

func TestDoCmdFunctionInvokeWithYAMLInputFlagsNoInputFile(t *testing.T) {
	t.Parallel()

	converterCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func(_ string) (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("my-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
			}, t.TempDir(), nil
		},
	}
	yamlHost := func(_ context.Context, d, statusD diag.Sink) (plugin.Host, error) {
		return &plugin.MockHost{
			LoaderF: func(ctx *plugin.Context) (*plugin.GrpcServer, error) {
				return plugin.NewServer(ctx, schema.LoaderRegistration(schema.NewLoaderServerFromContext(ctx)))
			},
		}, nil
	}
	loadConverter := func(
		_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(ctx context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				converterCalled = true
				assert.Equal(t, "<no input file>", req.Filename)
				assert.Empty(t, req.Source)
				assert.NotEmpty(t, req.TargetLoader)
				assert.Equal(t, "azure:index:myFunction", req.Token)
				assert.Equal(t, map[string]string{"message": "instring"}, req.Attributes)
				return &plugin.ConvertSnippetResponse{
					Filename: "inputs.pp",
					Attributes: map[string]string{
						"message": "\"outstring\"",
					},
				}, nil
			},
		}, nil
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"message": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
						Required: []string{"message"},
					},
					Outputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"output1": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
					},
				},
			},
		}
		return &testProvider{
			spec: spec,
			MockProvider: plugin.MockProvider{
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "outstring", req.Args["message"].StringValue())
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{"output1": resource.NewProperty("world")},
					}, nil
				},
			},
		}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, yamlHost, loadConverter)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"azure:index:myFunction",
		"--input", "yaml",
		"--input:message", "instring",
	})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, converterCalled, "ConvertSnippet should be called for YAML flags even without --input-file")
	assert.JSONEq(t, `{"output1": "world"}`, stdout.String())
}
