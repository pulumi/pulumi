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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoCmdWithFunctionHelpArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myModule", "myOtherFunction", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Invoke the myOtherFunction function.

This is the other function in this package.

Inputs:
  param1 (string, required) - To set param1 things
  param2 (Array<number>, optional) - Optional values.

Outputs:
  output1 (string, required) - The first output.
  output2 (number, optional) - The second output.
  output3 (boolean, required) - Whether it worked.

Usage:
  do azure myModule myOtherFunction [flags]

Flags:
  -h, --help                help for myOtherFunction
      --input-file string   Path to a file containing function inputs

Global Flags:
      --dry-run                Run the operation in preview mode
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvoke(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "details": "[secret]",
  "name": "visible"
}
`
	assert.Equal(t, expected, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"azure", "myFunction", "--show-secrets"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param = "hello"
`)

	stdout.Reset()
	cmd.SetArgs([]string{"pkg", "mod1/mod2", "fun", "--input-file", inputFile})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
	err := cmd.Execute()
	require.ErrorContains(t, err, `Missing required input "param1"`)
}

// TestDoCmdFunctionInvoke_NoInputFileWithRequired asserts that invoking a function which declares required inputs but
// without --input-file is rejected before the provider is called.
func TestDoCmdFunctionInvoke_NoInputFileWithRequired(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure", "myFunction"})
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
			loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
				return closer(t), &testProvider{
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
			cmd := NewDoCmd(mlm, mws, loader)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)

			inputFile := writeHCLFile(t, "inputs.pcl", tt.input)
			cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported attribute 'bogus'")
}

func TestDoCmdFunctionInvokeInputFileSchemaConversions(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = 44
param2 = "true"
param3 = "45"
`)

	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	inputFile := writeHCLFile(t, "inputs.pcl", `
param1 = "hello"
param2 = 42
param3 = true
`)

	cmd.SetArgs([]string{"--dry-run", "azure", "myFunction", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `{
  "output1": "world",
  "output2": 43,
  "output3": "<unknown>"
}
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdFunctionInvokeWithBuiltinFunctions(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
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
			loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
				return closer(t), &testProvider{spec: spec}, nil
			}

			var stdout bytes.Buffer
			cmd := NewDoCmd(mlm, mws, loader)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)

			inputFile := writeHCLFile(t, "inputs.pcl", tt.input)

			cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
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
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("my-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
				Main:    "infra",
			}, root, nil
		},
	}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDoCmdFunctionInvokeWithConfiguration(t *testing.T) {
	t.Parallel()

	configureCalled := false
	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		spec := schema.PackageSpec{
			Name: "azure",
			Provider: schema.ResourceSpec{
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
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
		"azure",
		"--provider-file", providerFile,
		"myFunction",
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"--show-secrets", "azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure", "myFunction"})
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{
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
	cmd := NewDoCmd(mlm, mws, loader)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	// First positional is the package spec: base provider name plus any Parameterize args, shlex-quoted.
	cmd.SetArgs([]string{"terraform-provider foo/bar 1.2.3", "myFunction", "--input-file", inputFile})
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
