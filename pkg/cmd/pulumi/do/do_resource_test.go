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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doResourceSpec(withList bool) schema.PackageSpec {
	res := schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: "A test resource.",
			Properties: map[string]schema.PropertySpec{
				"name":    {TypeSpec: schema.TypeSpec{Type: "string"}},
				"size":    {TypeSpec: schema.TypeSpec{Type: "integer"}},
				"enabled": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
				"extra":   {TypeSpec: schema.TypeSpec{Type: "string"}},
			},
		},
		InputProperties: map[string]schema.PropertySpec{
			"name": {
				TypeSpec:    schema.TypeSpec{Type: "string"},
				Description: "The resource name.",
			},
			"size": {
				TypeSpec: schema.TypeSpec{Type: "integer"},
			},
			"enabled": {
				TypeSpec: schema.TypeSpec{Type: "boolean"},
			},
		},
		RequiredInputs: []string{"name"},
	}
	if withList {
		res.ListInputs = &schema.ObjectTypeSpec{
			Properties: map[string]schema.PropertySpec{
				"prefix": {TypeSpec: schema.TypeSpec{Type: "string"}},
			},
		}
	}
	return schema.PackageSpec{
		Name: "azure",
		Resources: map[string]schema.ResourceSpec{
			"azure:index:myResource": res,
		},
	}
}

func newDoResourceCommand(
	t *testing.T, provider *testProvider,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return provider, nil
	}

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

func TestDoCmdResourceHelpListsOperations(t *testing.T) {
	t.Parallel()

	cmd, stdout, _ := newDoResourceCommand(t, &testProvider{spec: doResourceSpec(true)})
	cmd.SetArgs([]string{"azure:index:myResource", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	//nolint:lll
	expected := `Operate on the myResource resource.

A test resource.

Inputs:
  enabled (boolean, optional)
  name (string, required) - The resource name.
  size (integer, optional)

Outputs:
  enabled (boolean)
  extra (string)
  name (string)
  size (integer)

List Inputs:
  prefix (string, optional)

Usage:
  do azure:index:myResource [flags]
  do azure:index:myResource [command]

Available Commands:
  create      Create a resource
  delete      Delete a resource
  list        List resources
  patch       Patch a resource
  read        Read a resource

Flags:
      --dry-run                Run the operation in preview mode
  -h, --help                   help for do
      --input string           Format of the provider configuration file (default "pcl")
      --package string         The package to load, in the form 'name@version' or a path to a plugin binary or folder. If the package supports parameterization, additional space-separated parameters can be included after the package name, e.g. --package "name@version param1 \"multi word param\""
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output
      --stateless              Run create/patch/delete directly against the provider without persisting state. Required for now: the stateful (engine-driven) implementation is still in development, so create/patch/delete error out unless --stateless is set.

Use "do azure:index:myResource [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdResourceHelpOmitsListWithoutListInputs(t *testing.T) {
	t.Parallel()

	cmd, stdout, _ := newDoResourceCommand(t, &testProvider{spec: doResourceSpec(false)})
	cmd.SetArgs([]string{"azure:index:myResource", "--help"})
	err := cmd.Execute()
	require.NoError(t, err)

	//nolint:lll
	expected := `Operate on the myResource resource.

A test resource.

Inputs:
  enabled (boolean, optional)
  name (string, required) - The resource name.
  size (integer, optional)

Outputs:
  enabled (boolean)
  extra (string)
  name (string)
  size (integer)

Usage:
  do azure:index:myResource [flags]
  do azure:index:myResource [command]

Available Commands:
  create      Create a resource
  delete      Delete a resource
  patch       Patch a resource
  read        Read a resource

Flags:
      --dry-run                Run the operation in preview mode
  -h, --help                   help for do
      --input string           Format of the provider configuration file (default "pcl")
      --package string         The package to load, in the form 'name@version' or a path to a plugin binary or folder. If the package supports parameterization, additional space-separated parameters can be included after the package name, e.g. --package "name@version param1 \"multi word param\""
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output
      --stateless              Run create/patch/delete directly against the provider without persisting state. Required for now: the stateful (engine-driven) implementation is still in development, so create/patch/delete error out unless --stateless is set.

Use "do azure:index:myResource [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdResourceCreate(t *testing.T) {
	t.Parallel()

	var calls []string
	cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
		spec: doResourceSpec(false),
		MockProvider: plugin.MockProvider{
			CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
				calls = append(calls, "check")
				assert.Equal(t, tokens.Type("azure:index:myResource"), req.Type)
				assert.Equal(t, "example", req.News["name"].StringValue())
				assert.Equal(t, 2.0, req.News["size"].NumberValue())
				return plugin.CheckResponse{Properties: req.News}, nil
			},
			CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
				calls = append(calls, "create")
				assert.Equal(t, "example", req.Properties["name"].StringValue())
				return plugin.CreateResponse{
					ID: "res-1",
					Properties: resource.PropertyMap{
						"name":  resource.NewProperty("example"),
						"size":  resource.NewProperty(2.0),
						"extra": resource.NewProperty("hidden"),
					},
				}, nil
			},
		},
	})

	inputFile := writeHCLFile(t, "inputs.pcl", `
name = "example"
size = 2
`)
	cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "create", "--yes", "--input-file", inputFile})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, []string{"check", "create"}, calls)
	assert.JSONEq(t, `{
  "id": "res-1",
  "name": "example",
  "size": 2,
  "extra": "hidden"
}`, stdout.String())
}

func TestDoCmdResourceCreateWithPCLInputFlags(t *testing.T) {
	t.Parallel()

	spec := schema.PackageSpec{
		Name: "azure",
		Resources: map[string]schema.ResourceSpec{
			"azure:index:myResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"name":               {TypeSpec: schema.TypeSpec{Type: "string"}},
						"intValue":           {TypeSpec: schema.TypeSpec{Type: "integer"}},
						"already-kebab-case": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"snake_case":         {TypeSpec: schema.TypeSpec{Type: "boolean"}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"name":               {TypeSpec: schema.TypeSpec{Type: "string"}},
					"intValue":           {TypeSpec: schema.TypeSpec{Type: "integer"}},
					"already-kebab-case": {TypeSpec: schema.TypeSpec{Type: "string"}},
					"snake_case":         {TypeSpec: schema.TypeSpec{Type: "boolean"}},
				},
				RequiredInputs: []string{"name"},
			},
		},
	}

	cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
		spec: spec,
		MockProvider: plugin.MockProvider{
			CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
				assert.Equal(t, "example", req.News["name"].StringValue())
				assert.Equal(t, 42.0, req.News["intValue"].NumberValue())
				assert.Equal(t, "kebab", req.News["already-kebab-case"].StringValue())
				assert.Equal(t, true, req.News["snake_case"].BoolValue())
				return plugin.CheckResponse{Properties: req.News}, nil
			},
			CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{
					ID:         "res-1",
					Properties: req.Properties,
				}, nil
			},
		},
	})

	inputFile := writeHCLFile(t, "inputs.pcl", `name = "example"`)
	cmd.SetArgs([]string{
		"--stateless",
		"azure:index:myResource", "create",
		"--yes",
		"--input-file", inputFile,
		"--int-value", "42",
		"--already-kebab-case", "kebab",
		"--snake-case",
	})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.JSONEq(t, `{
  "id": "res-1",
  "name": "example",
  "intValue": 42,
  "already-kebab-case": "kebab",
  "snake_case": true
}`, stdout.String())
}

func TestDoCmdResourceReadDeletePatch(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					assert.Equal(t, resource.ID("res-1"), req.ID)
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID: "res-1",
							Outputs: resource.PropertyMap{
								"name": resource.NewProperty("read"),
								"size": resource.NewProperty(3.0),
							},
						},
					}, nil
				},
			},
		})
		cmd.SetArgs([]string{"azure:index:myResource", "read", "res-1"})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":"res-1","name":"read","size":3}`, stdout.String())
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		var deleted bool
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				DeleteF: func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = true
					assert.Equal(t, resource.ID("res-1"), req.ID)
					assert.Empty(t, req.Inputs)
					assert.Empty(t, req.Outputs)
					return plugin.DeleteResponse{}, nil
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "res-1", "--yes"})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.True(t, deleted)
		assert.Empty(t, stdout.String())
	})

	t.Run("patch", func(t *testing.T) {
		t.Parallel()
		var calls []string
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					calls = append(calls, "read")
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID: "res-1",
							Inputs: resource.PropertyMap{
								"name":    resource.NewProperty("old"),
								"size":    resource.NewProperty(1.0),
								"enabled": resource.NewProperty(false),
							},
							Outputs: resource.PropertyMap{
								"name":    resource.NewProperty("old"),
								"size":    resource.NewProperty(1.0),
								"enabled": resource.NewProperty(false),
							},
						},
					}, nil
				},
				CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					calls = append(calls, "check")
					assert.Equal(t, "old", req.Olds["name"].StringValue())
					assert.Equal(t, "new", req.News["name"].StringValue())
					assert.Equal(t, 1.0, req.News["size"].NumberValue())
					assert.Equal(t, true, req.News["enabled"].BoolValue())
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					calls = append(calls, "diff")
					return plugin.DiffResponse{
						Changes:     plugin.DiffSome,
						ChangedKeys: []resource.PropertyKey{"name", "enabled"},
					}, nil
				},
				UpdateF: func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					calls = append(calls, "update")
					assert.Equal(t, "new", req.NewInputs["name"].StringValue())
					assert.Equal(t, 1.0, req.NewInputs["size"].NumberValue())
					assert.Equal(t, true, req.NewInputs["enabled"].BoolValue())
					return plugin.UpdateResponse{
						Properties: resource.PropertyMap{
							"name":    resource.NewProperty("new"),
							"size":    resource.NewProperty(1.0),
							"enabled": resource.NewProperty(true),
						},
					}, nil
				},
			},
		})

		inputFile := writeHCLFile(t, "patch.pcl", `
name = "new"
enabled = true
`)
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "patch", "res-1", "--yes", "--input-file", inputFile})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.Equal(t, []string{"read", "check", "diff", "update"}, calls)
		assert.JSONEq(t, `{"id":"res-1","name":"new","size":1,"enabled":true}`, stdout.String())
	})

	// Partial patches should not require the user to restate required inputs that haven't changed; the existing
	// inputs from Read fill those in, and the patch file is bound with AllowMissingProperties.
	t.Run("patch omitting required input", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID: "res-1",
							Inputs: resource.PropertyMap{
								"name":    resource.NewProperty("existing"),
								"enabled": resource.NewProperty(false),
							},
							Outputs: resource.PropertyMap{
								"name":    resource.NewProperty("existing"),
								"enabled": resource.NewProperty(false),
							},
						},
					}, nil
				},
				CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					assert.Equal(t, "existing", req.News["name"].StringValue())
					assert.Equal(t, true, req.News["enabled"].BoolValue())
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
				},
				UpdateF: func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: resource.PropertyMap{
							"name":    resource.NewProperty("existing"),
							"enabled": resource.NewProperty(true),
						},
					}, nil
				},
			},
		})

		inputFile := writeHCLFile(t, "patch.pcl", `enabled = true`)
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "patch", "res-1", "--yes", "--input-file", inputFile})
		err := cmd.Execute()
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":"res-1","name":"existing","enabled":true}`, stdout.String())
	})
}

func TestDoCmdResourceList(t *testing.T) {
	t.Parallel()

	t.Run("single page by default", func(t *testing.T) {
		t.Parallel()
		var calls []plugin.ListRequest
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(true),
			MockProvider: plugin.MockProvider{
				ListF: func(ctx context.Context, req plugin.ListRequest) (*plugin.ListStream, error) {
					calls = append(calls, req)
					assert.Equal(t, "prod", req.Query["prefix"].StringValue())
					return plugin.NewListStream([]plugin.ListResult{{ID: "1", Name: "one"}}, "next"), nil
				},
			},
		})
		inputFile := writeHCLFile(t, "list.pcl", `prefix = "prod"`)
		cmd.SetArgs([]string{"azure:index:myResource", "list", "--input-file", inputFile})
		err := cmd.Execute()
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Empty(t, calls[0].ContinuationToken)
		assert.JSONEq(t, `[{"id":"1","name":"one"}]`, stdout.String())
	})

	t.Run("count", func(t *testing.T) {
		t.Parallel()
		var calls []plugin.ListRequest
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(true),
			MockProvider: plugin.MockProvider{
				ListF: func(ctx context.Context, req plugin.ListRequest) (*plugin.ListStream, error) {
					calls = append(calls, req)
					if req.ContinuationToken == "" {
						return plugin.NewListStream([]plugin.ListResult{{ID: "1", Name: "one"}}, "next"), nil
					}
					return plugin.NewListStream(
						[]plugin.ListResult{{ID: "2", Name: "two"}, {ID: "3", Name: "three"}},
						"",
					), nil
				},
			},
		})
		cmd.SetArgs([]string{"azure:index:myResource", "list", "--count", "2"})
		err := cmd.Execute()
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, int64(2), calls[0].Limit)
		assert.Equal(t, int64(1), calls[1].Limit)
		assert.JSONEq(t, `[{"id":"1","name":"one"},{"id":"2","name":"two"}]`, stdout.String())
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()
		var calls []plugin.ListRequest
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(true),
			MockProvider: plugin.MockProvider{
				ListF: func(ctx context.Context, req plugin.ListRequest) (*plugin.ListStream, error) {
					calls = append(calls, req)
					if req.ContinuationToken == "" {
						return plugin.NewListStream([]plugin.ListResult{{ID: "1", Name: "one"}}, "next"), nil
					}
					return plugin.NewListStream([]plugin.ListResult{{ID: "2", Name: "two"}}, ""), nil
				},
			},
		})
		cmd.SetArgs([]string{"azure:index:myResource", "list", "--all"})
		err := cmd.Execute()
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, "next", calls[1].ContinuationToken)
		assert.JSONEq(t, `[{"id":"1","name":"one"},{"id":"2","name":"two"}]`, stdout.String())
	})

	t.Run("mutually exclusive flags", func(t *testing.T) {
		t.Parallel()
		cmd, _, _ := newDoResourceCommand(t, &testProvider{spec: doResourceSpec(true)})
		cmd.SetArgs([]string{"azure:index:myResource", "list", "--all", "--count", "1"})
		err := cmd.Execute()
		require.ErrorContains(t, err, "--all and --count are mutually exclusive")
	})
}

// TestDoCmdResourceNonInteractiveRequiresYes asserts that destructive subcommands refuse to run when the user is
// not on a TTY and --yes was not supplied. Tests have no TTY, so omitting --yes triggers the early bail.
func TestDoCmdResourceNonInteractiveRequiresYes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"create", []string{"--stateless", "azure:index:myResource", "create"}},
		{"patch", []string{"--stateless", "azure:index:myResource", "patch", "res-1"}},
		{"delete", []string{"--stateless", "azure:index:myResource", "delete", "res-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &testProvider{
				spec: doResourceSpec(false),
				MockProvider: plugin.MockProvider{
					CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						require.Fail(t, "Create should not be called without --yes in non-interactive mode")
						return plugin.CreateResponse{}, nil
					},
					DeleteF: func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
						require.Fail(t, "Delete should not be called without --yes in non-interactive mode")
						return plugin.DeleteResponse{}, nil
					},
					UpdateF: func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
						require.Fail(t, "Update should not be called without --yes in non-interactive mode")
						return plugin.UpdateResponse{}, nil
					},
				},
			}
			cmd, _, _ := newDoResourceCommand(t, provider)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
		})
	}
}

// TestDoCmdResourceConfirmationSummary asserts the operation summary lands on stderr (so stdout stays a clean
// JSON channel for piping) and that the patch summary surfaces the Diff response.
func TestDoCmdResourceConfirmationSummary(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "res-1", Properties: req.Properties}, nil
				},
			},
		})
		inputFile := writeHCLFile(t, "inputs.pcl", `name = "example"`)
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "create", "--yes", "--input-file", inputFile})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "This will create azure:index:myResource")
		assert.NotContains(t, stdout.String(), "This will create")
	})

	t.Run("patch surfaces diff", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      "res-1",
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("old")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("old")},
					}}, nil
				},
				CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					return plugin.DiffResponse{
						Changes: plugin.DiffSome,
						DetailedDiff: map[string]plugin.PropertyDiff{
							"name": {Kind: plugin.DiffUpdate},
						},
					}, nil
				},
				UpdateF: func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{Properties: req.NewInputs}, nil
				},
			},
		})
		inputFile := writeHCLFile(t, "patch.pcl", `name = "new"`)
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "patch", "res-1", "--yes", "--input-file", inputFile})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "This will update azure:index:myResource")
		assert.Contains(t, stderr.String(), "~ name")
		assert.NotContains(t, stdout.String(), "This will update")
	})

	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				DeleteF: func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, nil
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "res-1", "--yes"})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), `This will delete azure:index:myResource "res-1"`)
		assert.Empty(t, stdout.String())
	})
}
