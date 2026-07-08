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
	"errors"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
      --input string           Format of the provider configuration file (default "yaml")
      --output string          Output format for resource operation results (supported: default, json)
      --package string         The package to load, in the form 'name@version' or a path to a plugin binary or folder. If the package supports parameterization, additional space-separated parameters can be included after the package name, e.g. --package "name@version param1 \"multi word param\""
      --provider string        The URN of a provider resource in the current stack whose inputs to use as the base of the provider configuration (requires a stack context)
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
  do azure:index:myResource [command]

Available Commands:
  create      Create a resource
  delete      Delete a resource
  patch       Patch a resource
  read        Read a resource

Flags:
      --dry-run                Run the operation in preview mode
  -h, --help                   help for do
      --input string           Format of the provider configuration file (default "yaml")
      --output string          Output format for resource operation results (supported: default, json)
      --package string         The package to load, in the form 'name@version' or a path to a plugin binary or folder. If the package supports parameterization, additional space-separated parameters can be included after the package name, e.g. --package "name@version param1 \"multi word param\""
      --provider string        The URN of a provider resource in the current stack whose inputs to use as the base of the provider configuration (requires a stack context)
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
	cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
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
	cmd.SetArgs([]string{
		"--stateless", "azure:index:myResource", "create", "--yes",
		"--input", "pcl", "--input-file", inputFile, "--output", "json",
	})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, []string{"check", "create"}, calls)
	assert.JSONEq(t, `{
  "id": "res-1",
  "name": "example",
  "size": 2,
  "extra": "hidden"
}`, stdout.String())
	assert.NotContains(t, stderr.String(), "creating")
	assert.NotContains(t, stderr.String(), "Outputs:")
	assert.NotContains(t, stderr.String(), "Resources:")
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
		"--output", "json",
		"--input", "pcl",
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
		cmd.SetArgs([]string{"azure:index:myResource", "read", "res-1", "--output", "json"})
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
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "patch", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile, "--output", "json",
		})
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
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "patch", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile, "--output", "json",
		})
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
		cmd.SetArgs([]string{"azure:index:myResource", "list", "--input", "pcl", "--input-file", inputFile})
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

// TestDoCmdResourceDeleteDryRun asserts that delete --dry-run prints what would be deleted and never
// calls the provider. Delete has no provider-side preview mode, so the summary is the whole dry run;
// crucially --dry-run must not act like --yes (the confirmation prompt is skipped on dry runs).
func TestDoCmdResourceDeleteDryRun(t *testing.T) {
	t.Parallel()

	cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
		spec: doResourceSpec(false),
		MockProvider: plugin.MockProvider{
			DeleteF: func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
				require.Fail(t, "Delete should not be called with --dry-run")
				return plugin.DeleteResponse{}, nil
			},
		},
	})
	cmd.SetArgs([]string{"--dry-run", "--stateless", "azure:index:myResource", "delete", "res-1"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), `This would delete azure:index:myResource "res-1"`)
	assert.Empty(t, stdout.String())
}

// TestDoCmdResourceDryRunIgnoredForReadOnlyOps asserts that read and list ignore --dry-run: they never
// mutate anything, so there is nothing to preview and they behave as if the flag wasn't passed.
func TestDoCmdResourceDryRunIgnoredForReadOnlyOps(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Outputs: resource.PropertyMap{"name": resource.NewProperty("read")},
						},
					}, nil
				},
			},
		})
		cmd.SetArgs([]string{"--dry-run", "azure:index:myResource", "read", "res-1", "--output=json"})
		require.NoError(t, cmd.Execute())
		assert.JSONEq(t, `{"id":"res-1","name":"read"}`, stdout.String())
	})

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(true),
			MockProvider: plugin.MockProvider{
				ListF: func(ctx context.Context, req plugin.ListRequest) (*plugin.ListStream, error) {
					return plugin.NewListStream([]plugin.ListResult{{ID: "1", Name: "one"}}, ""), nil
				},
			},
		})
		cmd.SetArgs([]string{"--dry-run", "azure:index:myResource", "list"})
		require.NoError(t, cmd.Execute())
		assert.JSONEq(t, `[{"id":"1","name":"one"}]`, stdout.String())
	})
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
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "create", "--yes",
			"--input", "pcl", "--input-file", inputFile,
		})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "This will create azure:index:myResource")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource creating")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource created")
		assert.Contains(t, stderr.String(), "+ 1 created")
		assert.NotContains(t, stderr.String(), "pulumi:pulumi:Stack")
		assert.Contains(t, stderr.String(), "Outputs:")
		assert.Contains(t, stderr.String(), `"example"`)
		assert.Contains(t, stderr.String(), `"res-1"`)
		assert.Empty(t, stdout.String())
	})

	t.Run("create failure", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				CheckF: func(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				CreateF: func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("quota exceeded")
				},
			},
		})
		inputFile := writeHCLFile(t, "inputs.pcl", `name = "example"`)
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "create", "--yes",
			"--input", "pcl", "--input-file", inputFile,
		})
		err := cmd.Execute()
		assert.ErrorContains(t, err, "quota exceeded")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource creating")
		assert.Contains(t, stderr.String(), "failed")
		assert.NotContains(t, stdout.String(), "creating")
		assert.NotContains(t, stdout.String(), "failed")
	})

	t.Run("read", func(t *testing.T) {
		t.Parallel()
		cmd, stdout, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      "res-1",
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("read")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("read")},
					}}, nil
				},
			},
		})
		cmd.SetArgs([]string{"azure:index:myResource", "read", "res-1"})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource read")
		assert.Contains(t, stderr.String(), "Outputs:")
		assert.Contains(t, stderr.String(), `"read"`)
		assert.Contains(t, stderr.String(), `"res-1"`)
		assert.NotContains(t, stderr.String(), "pulumi:pulumi:Stack")
		assert.NotContains(t, stderr.String(), "Resources:")
		assert.Empty(t, stdout.String())
	})

	t.Run("read masks secrets", func(t *testing.T) {
		t.Parallel()
		cmd, _, stderr := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      "res-1",
						Inputs:  resource.PropertyMap{},
						Outputs: resource.PropertyMap{"name": resource.MakeSecret(resource.NewProperty("hunter2"))},
					}}, nil
				},
			},
		})
		cmd.SetArgs([]string{"azure:index:myResource", "read", "res-1"})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "[secret]")
		assert.NotContains(t, stderr.String(), "hunter2")
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
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "patch", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile,
		})
		require.NoError(t, cmd.Execute())
		assert.Contains(t, stderr.String(), "This will update azure:index:myResource")
		assert.Contains(t, stderr.String(), "~ name")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource updating")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource updated")
		assert.Contains(t, stderr.String(), "[diff: ~name]")
		assert.Contains(t, stderr.String(), "~ 1 updated")
		changesIdx := strings.Index(stderr.String(), "Changes:")
		outputsIdx := strings.Index(stderr.String(), "Outputs:")
		require.GreaterOrEqual(t, changesIdx, 0)
		require.Greater(t, outputsIdx, changesIdx)
		changes := stderr.String()[changesIdx:outputsIdx]
		assert.Contains(t, changes, `~ name: "old" => "new"`)
		assert.Contains(t, stderr.String()[outputsIdx:], `"res-1"`)
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
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource deleting")
		assert.Contains(t, stderr.String(), "azure:index:myResource myResource deleted")
		assert.Contains(t, stderr.String(), "- 1 deleted")
		assert.Empty(t, stdout.String())
	})
}

// TestDoCmdResourceProviderFlagOutsideStackContext checks that the --provider flag errors out when
// there is no stack context to resolve the URN against — passing it when no Pulumi project is on
// disk (the default MockContext returns ErrProjectNotFound) should fail with a clear message
// before any provider configuration happens.
//
// don't bother mocking the provider; the bail also happens regardless of subcommand, so we use
// `read` for brevity.
func TestDoCmdResourceProviderFlagOutsideStackContext(t *testing.T) {
	t.Parallel()
	cmd, _, _ := newDoResourceCommand(t, &testProvider{spec: doResourceSpec(false)})
	cmd.SetArgs([]string{
		"azure:index:myResource", "read", "res-1",
		"--provider", "urn:pulumi:dev::proj::pulumi:providers:azure::default",
	})
	err := cmd.Execute()
	require.ErrorContains(t, err, "--provider requires a stack context")
}

// providerFlagStackContext wires a `do` command up against a mocked workspace + backend so that
// configureProvider's RequireStack → CurrentBackend → CurrentStack chain finds a stack whose
// snapshot is exactly `snapshot`. Returns the cmd plus output buffers. The fully-qualified stack
// name avoids tripping getStackNameWithLegacyOrgNameIfNeeded, which would otherwise call into the
// MockBackend trying to look up a default org.
func providerFlagStackContext(
	t *testing.T, provider *testProvider, snapshot *deploy.Snapshot,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	// state.CurrentStack consults PULUMI_STACK ahead of the workspace's stored selection. Set it
	// to a fully-qualified name so getStackNameWithLegacyOrgNameIfNeeded skips its default-org
	// lookup (which would otherwise call into the MockBackend).
	t.Setenv("PULUMI_STACK", "myorg/proj/dev")

	proj := &workspace.Project{Name: tokens.PackageName("proj")}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return proj, t.TempDir(), nil
		},
		// `do` populates evalContext.Stack from ws.New().Settings().Stack via currentStackIdentity;
		// that determines whether configureProvider considers us "in a stack context". PULUMI_STACK
		// above only feeds the parallel path used by state.CurrentStack — both have to agree.
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "myorg/proj/dev"}
				},
			}, nil
		},
	}

	stackRef := &backend.MockStackReference{
		StringV:             "myorg/proj/dev",
		NameV:               tokens.MustParseStackName("dev"),
		FullyQualifiedNameV: "myorg/proj/dev",
	}
	mockStack := &backend.MockStack{
		RefF: func() backend.StackReference { return stackRef },
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
	}
	mockBackend := &backend.MockBackend{
		ParseStackReferenceF: func(_ string) (backend.StackReference, error) { return stackRef, nil },
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return mockStack, nil
		},
	}
	mlm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool,
			bool, colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return provider, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

// TestDoCmdResourceProviderFlagURNNotInSnapshot verifies that --provider names a URN that does not
// match any resource in the current stack's snapshot fails with a "no resource named" error,
// rather than silently configuring the provider with an empty / default config.
//
//nolint:paralleltest // mutates cmdBackend.BackendInstance via providerFlagStackContext.
func TestDoCmdResourceProviderFlagURNNotInSnapshot(t *testing.T) {
	cmd, _, _ := providerFlagStackContext(t,
		&testProvider{spec: doResourceSpec(false)},
		&deploy.Snapshot{}, // empty snapshot — no resources
	)
	cmd.SetArgs([]string{
		"azure:index:myResource", "read", "res-1",
		"--provider", "urn:pulumi:dev::proj::pulumi:providers:azure::default",
	})
	err := cmd.Execute()
	require.ErrorContains(t, err, "no resource named")
	require.ErrorContains(t, err, "urn:pulumi:dev::proj::pulumi:providers:azure::default")
}

// TestDoCmdResourceProviderFlagURNNotAProvider verifies that --provider only accepts a URN
// pointing at a provider resource for the *same* package as the command's target. Two failure
// modes guarded here:
//
//   - "not a provider": the URN names something whose type doesn't start with `pulumi:providers:`
//     (e.g. an azure:index:Bucket). Without this guard we'd hand a Bucket's inputs to
//     provider.Configure and get a confusing schema mismatch.
//   - "wrong package": the URN names a provider for a different package (e.g. pointing at an aws
//     provider while running `pulumi do azure:...`). Worst case here is a silent
//     misconfiguration where Configure happily takes the cross-cloud inputs and we authenticate
//     against the wrong cloud — definitely a loud-fail case.
//
//nolint:paralleltest // see TestDoCmdResourceProviderFlagURNNotInSnapshot.
func TestDoCmdResourceProviderFlagURNNotAProvider(t *testing.T) {
	bucketURN := resource.URN("urn:pulumi:dev::proj::azure:index:Bucket::mybucket")
	awsProviderURN := resource.URN("urn:pulumi:dev::proj::pulumi:providers:aws::default")

	cases := []struct {
		name       string
		urn        resource.URN
		resources  []*resource.State
		wantSubstr string
	}{
		{
			name: "not a provider",
			urn:  bucketURN,
			//nolint:requiredfield // Only the fields configureProvider's matcher reads matter here.
			resources: []*resource.State{
				(&resource.NewState{
					Type:   "azure:index:Bucket",
					URN:    bucketURN,
					Custom: true,
					Inputs: resource.PropertyMap{"region": resource.NewProperty("us-east-1")},
				}).Make(),
			},
			wantSubstr: "is not a provider",
		},
		{
			name: "provider for a different package",
			urn:  awsProviderURN,
			//nolint:requiredfield // Only the fields configureProvider's matcher reads matter here.
			resources: []*resource.State{
				(&resource.NewState{
					Type:   "pulumi:providers:aws",
					URN:    awsProviderURN,
					Custom: true,
					Inputs: resource.PropertyMap{"region": resource.NewProperty("us-east-1")},
				}).Make(),
			},
			wantSubstr: "provider for a different package",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _, _ := providerFlagStackContext(t,
				&testProvider{spec: doResourceSpec(false)},
				&deploy.Snapshot{Resources: tc.resources},
			)
			cmd.SetArgs([]string{
				"azure:index:myResource", "read", "res-1",
				"--provider", string(tc.urn),
			})
			err := cmd.Execute()
			require.ErrorContains(t, err, tc.wantSubstr)
		})
	}
}

// TestDoCmdResourceProviderFlagMergesStackInputs is the positive path: --provider points at a real
// provider resource in the snapshot, and its Inputs are used as the base. The CLI --input:region
// flag overlays on top, so the value the provider receives at Configure time is the overlay
// (us-west-2) — proving that explicit user-supplied values win, while values absent from the
// overlay (the snapshot's `tenant`) fall through unchanged.
//
//nolint:paralleltest // see TestDoCmdResourceProviderFlagURNNotInSnapshot.
func TestDoCmdResourceProviderFlagMergesStackInputs(t *testing.T) {
	providerURN := resource.URN("urn:pulumi:dev::proj::pulumi:providers:azure::shared")
	// The persistent --<pkg>:<input> flags are driven by spec.Provider.InputProperties; the regular
	// doResourceSpec helper doesn't populate that, so add region/tenant here for this test only.
	spec := doResourceSpec(false)
	spec.Provider = &schema.ResourceSpec{
		InputProperties: map[string]schema.PropertySpec{
			"region": {TypeSpec: schema.TypeSpec{Type: "string"}},
			"tenant": {TypeSpec: schema.TypeSpec{Type: "string"}},
		},
	}
	var gotInputs resource.PropertyMap
	cmd, _, _ := providerFlagStackContext(t, &testProvider{
		spec: spec,
		MockProvider: plugin.MockProvider{
			ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
				gotInputs = req.Inputs
				return plugin.ConfigureResponse{}, nil
			},
			ReadF: func(_ context.Context, _ plugin.ReadRequest) (plugin.ReadResponse, error) {
				return plugin.ReadResponse{ReadResult: plugin.ReadResult{
					ID:      "res-1",
					Outputs: resource.PropertyMap{"name": resource.NewProperty("hello")},
				}}, nil
			},
		},
	},
		//nolint:requiredfield // Only the fields configureProvider's matcher reads matter here.
		&deploy.Snapshot{Resources: []*resource.State{
			(&resource.NewState{
				Type:   "pulumi:providers:azure",
				URN:    providerURN,
				Custom: true,
				Inputs: resource.PropertyMap{
					"region": resource.NewProperty("us-east-1"),
					"tenant": resource.NewProperty("acme"),
				},
			}).Make(),
		}},
	)
	// --azure:region overlays the snapshot's `region` (the provider package's namespace flag form),
	// `tenant` is not supplied here and should fall through from the snapshot's inputs.
	cmd.SetArgs([]string{
		"azure:index:myResource", "read", "res-1",
		"--provider", string(providerURN),
		"--azure:region", "us-west-2",
	})
	require.NoError(t, cmd.Execute())

	require.NotNil(t, gotInputs, "provider.Configure should have been called")
	assert.Equal(t, "us-west-2", gotInputs["region"].StringValue(), "overlay should win for explicitly-set keys")
	assert.Equal(t, "acme", gotInputs["tenant"].StringValue(),
		"snapshot value should pass through for keys not in overlay")
}
