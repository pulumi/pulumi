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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panicLoadConverterPlugin(
	*plugin.Context, string, func(sev diag.Severity, msg string),
) (plugin.Converter, error) {
	panic("unexpected call to load converter plugin")
}

func panicLoader(context.Context, *plugin.Context, string, string) (plugin.Provider, error) {
	panic("not implemented")
}

func testHost() (plugin.Host, error) {
	return &plugin.MockHost{}, nil
}

func TestDoCmdNoArgsPrintsHelp(t *testing.T) {
	t.Parallel()

	table := []struct {
		name string
		args []string
	}{
		{name: "no args", args: []string{}},
		{name: "with --help", args: []string{"--help"}},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mlm := &cmdBackend.MockLoginManager{}
			mws := &pkgWorkspace.MockContext{}

			var stdout bytes.Buffer
			cmd := NewDoCmd(mlm, mws, panicLoader, testHost, panicLoadConverterPlugin)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.NoError(t, err)

			output := stdout.String()
			assert.Contains(t, output, "Interact with any cloud")
			assert.Contains(t, output, "Usage:\n  do <pkg:mod:typ> [command] [flags]\n")
		})
	}
}

type testProvider struct {
	plugin.MockProvider
	spec schema.PackageSpec
}

// Default to a no-op Configure function to avoid panics in tests that don't require provider configuration.
func (p *testProvider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	if p.ConfigureF != nil {
		return p.ConfigureF(ctx, req)
	}
	return plugin.ConfigureResponse{}, nil
}

func (p *testProvider) GetSchema(ctx context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	if p.GetSchemaF != nil {
		return p.GetSchemaF(ctx, req)
	}
	schemaBytes, err := json.Marshal(p.spec)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	return plugin.GetSchemaResponse{Schema: schemaBytes}, nil
}

func writeHCLFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestDoCmdWithPkgFlagPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws@4.1", source)
		spec := schema.PackageSpec{
			Name:        "aws",
			Description: "Help text about aws.",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
			Provider: schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"region": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
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

	cmd.SetArgs([]string{"--package", "aws@4.1"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Interact with aws resources and functions.

Help text about aws.

Run 'pulumi do --package aws@4.1 <module/resource/function> --help' for more details on usage.

Modules:
  aws:myModule

Functions:
  aws:myFunction

Resources:
  aws:myResource

`
	assert.Equal(t, expected, stdout.String())

	// Ensure that extra flags don't confuse the help message.
	stdout.Reset()
	cmd.SetArgs([]string{"--dry-run", "--package", "aws@4.1"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdWithPkgArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws", source)
		spec := schema.PackageSpec{
			Name:        "aws",
			Description: "Help text about aws.",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
			Provider: schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"region": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
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

	cmd.SetArgs([]string{"aws"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Interact with aws resources and functions.

Help text about aws.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Modules:
  aws:myModule

Functions:
  aws:myFunction

Resources:
  aws:myResource

`
	assert.Equal(t, expected, stdout.String())
}

// TestDoCmdWithPkgArgPrintsHelpWithModuleFormat asserts that when a package declares a non-default ModuleFormat
// — for example a bridged provider whose tokens look like "aws:s3/getAccessPoint:getAccessPoint" — the package
// help lists the simplified module ("aws:s3") rather than the raw module portion ("aws:s3/getAccessPoint").
// The simplification comes from schema.Package.TokenToModule honoring the format regex.
func TestDoCmdWithPkgArgPrintsHelpWithModuleFormat(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws", source)
		spec := schema.PackageSpec{
			Name:        "aws",
			Description: "Help text about aws.",
			Meta:        &schema.MetadataSpec{ModuleFormat: "(.*)(?:/[^/]*)"},
			Functions: map[string]schema.FunctionSpec{
				"aws:index/getArn:getArn":                                        {},
				"aws:s3/getAccessPoint:getAccessPoint":                           {},
				"aws:s3/getAccountPublicAccessBlock:getAccountPublicAccessBlock": {},
				"aws:ec2/getInstance:getInstance":                                {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index/object:Object":   {},
				"aws:s3/bucket:Bucket":      {},
				"aws:ec2/instance:Instance": {},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"aws"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	// Modules should be listed in their simplified form, one entry per unique top-level namespace, not the raw
	// "<pkg>:<ns>/<sub>" form.
	expected := `Interact with aws resources and functions.

Help text about aws.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Modules:
  aws:ec2
  aws:s3

Functions:
  aws:getArn

Resources:
  aws:Object

`
	assert.Equal(t, expected, output)

	// And ask for a module listing
	stdout.Reset()
	cmd.SetArgs([]string{"aws:s3"})
	err = cmd.Execute()
	require.NoError(t, err)

	output = stdout.String()
	expected = `Functions and resources for the s3 module.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Functions:
  aws:s3:getAccessPoint
  aws:s3:getAccountPublicAccessBlock

Resources:
  aws:s3:Bucket

`
	assert.Equal(t, expected, output)
}

// TestDoCmdWithPkgArgPrintsHelpSkipsMethods asserts that methods declared on a resource (which appear in
// PackageSpec.Functions but are bound with IsMethod=true) are *not* listed as standalone functions in the
// package or module help. Methods are reached via their owning resource, not as top-level callables.
func TestDoCmdWithPkgArgPrintsHelpSkipsMethods(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws", source)
		// Two functions: one is a regular invoke, the other is the implementation of a method on myResource.
		// Methods are referenced from ResourceSpec.Methods and must (1) have a token shaped
		// "<resource-token>/<method-name>" and (2) declare a __self__ input parameter.
		spec := schema.PackageSpec{
			Name:        "aws",
			Description: "Help text about aws.",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction": {},
				"aws:index:myResource/myMethod": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"__self__": {TypeSpec: schema.TypeSpec{Ref: "#/resources/aws:index:myResource"}},
						},
					},
				},
				"aws:myModule:myOtherFunction": {},
				"aws:myModule:myOtherResource/myOtherMethod": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"__self__": {TypeSpec: schema.TypeSpec{Ref: "#/resources/aws:myModule:myOtherResource"}},
						},
					},
				},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource": {
					Methods: map[string]string{"myMethod": "aws:index:myResource/myMethod"},
				},
				"aws:myModule:myOtherResource": {
					Methods: map[string]string{"myOtherMethod": "aws:myModule:myOtherResource/myOtherMethod"},
				},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	// Package-level help: methods (myMethod / myOtherMethod) should not appear under Functions.
	cmd.SetArgs([]string{"aws"})
	require.NoError(t, cmd.Execute())
	pkgHelp := stdout.String()
	assert.Contains(t, pkgHelp, "  aws:myFunction\n")
	assert.Contains(t, pkgHelp, "  aws:myResource\n")
	assert.Contains(t, pkgHelp, "  aws:myModule\n")
	assert.NotContains(t, pkgHelp, "myMethod")
	assert.NotContains(t, pkgHelp, "myOtherMethod")

	// Module-level help: same expectation — myOtherMethod should not be listed alongside myOtherFunction.
	stdout.Reset()
	cmd.SetArgs([]string{"aws:myModule"})
	require.NoError(t, cmd.Execute())
	modHelp := stdout.String()
	assert.Contains(t, modHelp, "  aws:myModule:myOtherFunction\n")
	assert.Contains(t, modHelp, "  aws:myModule:myOtherResource\n")
	assert.NotContains(t, modHelp, "myOtherMethod")
}

// TestDoCmdWithPkgArgPrintsHelpUnderRoot wraps the do command beneath a synthetic root with PersistentPreRun /
// PersistentPostRun (mimicking the real `pulumi` command). When the dynamic subcommand executes via
// subcmd.ExecuteContext, cobra walks back up to the root for a second Find/Execute pass — without the lifecycle
// bookkeeping in buildSubcommand that would re-run the root's persistent runs, sometimes panicking
// (e.g. the pulumi root's update-check goroutine closes a channel and a second send-after-close panics). Verify
// the persistent runs fire exactly once and the help output still matches.
func TestDoCmdWithPkgArgPrintsHelpUnderRoot(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws@4.1", source)
		spec := schema.PackageSpec{
			Name:        "aws",
			Description: "Help text about aws.",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
			Provider: schema.ResourceSpec{
				InputProperties: map[string]schema.PropertySpec{
					"region": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var preRunCount, postRunCount int
	rootCmd := &cobra.Command{
		Use: "pulumi",
		PersistentPreRun: func(*cobra.Command, []string) {
			preRunCount++
		},
		PersistentPostRun: func(*cobra.Command, []string) {
			postRunCount++
		},
	}
	rootCmd.AddCommand(NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin))

	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"do", "aws@4.1"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, 1, preRunCount, "PersistentPreRun should run exactly once across the nested Execute")
	assert.Equal(t, 1, postRunCount, "PersistentPostRun should run exactly once across the nested Execute")

	expected := `Interact with aws resources and functions.

Help text about aws.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Modules:
  aws:myModule

Functions:
  aws:myFunction

Resources:
  aws:myResource

`
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdWithModuleArgPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws@4.1", source)
		spec := schema.PackageSpec{
			Name: "aws",
			Functions: map[string]schema.FunctionSpec{
				"aws:index:myFunction":         {},
				"aws:myModule:myOtherFunction": {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:index:myResource":         {},
				"aws:myModule:myOtherResource": {},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"--package", "aws@4.1", "aws:myModule"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Functions and resources for the myModule module.

Run 'pulumi do --package aws@4.1 <module/resource/function> --help' for more details on usage.

Functions:
  aws:myModule:myOtherFunction

Resources:
  aws:myModule:myOtherResource

`
	assert.Equal(t, expected, stdout.String())

	// Ensure that extra flags don't confuse the help message.
	stdout.Reset()
	cmd.SetArgs([]string{"--dry-run", "--package", "aws@4.1", "aws:myModule"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, expected, stdout.String())
}

func TestDoCmdWithNestedModulesPrintsHelp(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "pkg", source)
		spec := schema.PackageSpec{
			Name: "pkg",
			Functions: map[string]schema.FunctionSpec{
				"pkg:mod1/mod2:fun": {},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"pkg"})
	err := cmd.Execute()
	require.NoError(t, err)

	expectedPackageHelp := `Interact with pkg resources and functions.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Modules:
  pkg:mod1/mod2

`
	assert.Equal(t, expectedPackageHelp, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"pkg:mod1", "--help"})
	err = cmd.Execute()
	require.NoError(t, err)

	expectedParentModuleHelp := `Functions and resources for the mod1 module.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Modules:
  pkg:mod1/mod2

`
	assert.Equal(t, expectedParentModuleHelp, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"pkg:mod1/mod2", "--help"})
	err = cmd.Execute()
	require.NoError(t, err)

	expectedNestedModuleHelp := `Functions and resources for the mod1/mod2 module.

Run 'pulumi do <module/resource/function> --help' for more details on usage.

Functions:
  pkg:mod1/mod2:fun

`
	assert.Equal(t, expectedNestedModuleHelp, stdout.String())

	// module isn't runnable so help should be printed even without --help flag
	stdout.Reset()
	cmd.SetArgs([]string{"pkg:mod1/mod2"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, expectedNestedModuleHelp, stdout.String())
}

func TestCurrentStackIdentity(t *testing.T) {
	t.Parallel()

	mockWS := func(stack string, newErr error) *pkgWorkspace.MockContext {
		return &pkgWorkspace.MockContext{
			NewF: func() (pkgWorkspace.W, error) {
				if newErr != nil {
					return nil, newErr
				}
				return &pkgWorkspace.MockW{
					SettingsF: func() *pkgWorkspace.Settings {
						return &pkgWorkspace.Settings{Stack: stack}
					},
				}, nil
			},
		}
	}

	table := []struct {
		name    string
		ws      pkgWorkspace.Context
		wantOrg string
		wantStk string
	}{
		{name: "fully qualified", ws: mockWS("acme/my-project/dev", nil), wantOrg: "acme", wantStk: "dev"},
		{name: "legacy two-part", ws: mockWS("acme/dev", nil), wantOrg: "acme", wantStk: "dev"},
		{name: "bare stack name", ws: mockWS("dev", nil), wantOrg: "", wantStk: "dev"},
		{name: "no stack selected", ws: mockWS("", nil), wantOrg: "", wantStk: ""},
		// New() can fail (e.g. no credentials file yet). The helper must not propagate that —
		// `do` should still run with no project context.
		{name: "workspace open fails", ws: mockWS("", errors.New("boom")), wantOrg: "", wantStk: ""},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			org, stk := currentStackIdentity(tc.ws)
			assert.Equal(t, tc.wantOrg, org)
			assert.Equal(t, tc.wantStk, stk)
		})
	}
}

// TestDoCmdFunctionInvokeWithStackContext exercises the end-to-end wiring: when a project is on disk
// and a stack is selected in the workspace, `do` should expose the organization and short stack name
// to PCL input files via the pulumi.organization / pulumi.stack builtins.
func TestDoCmdFunctionInvokeWithStackContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("my-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
			}, root, nil
		},
		NewF: func() (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "acme/my-project/dev"}
				},
			}, nil
		},
	}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		spec := schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"organization": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"project":      {TypeSpec: schema.TypeSpec{Type: "string"}},
							"stack":        {TypeSpec: schema.TypeSpec{Type: "string"}},
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
					assert.Equal(t, "acme", req.Args["organization"].StringValue())
					assert.Equal(t, "my-project", req.Args["project"].StringValue())
					assert.Equal(t, "dev", req.Args["stack"].StringValue())
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
organization = organization()
project = project()
stack = stack()
`)

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	cmd.SetArgs([]string{"azure:index:myFunction", "--input-file", inputFile})
	require.NoError(t, cmd.Execute())
}

// TestDoCmdFunctionInvokeWithoutStackContext asserts that when a project is present but no stack is
// selected in the workspace, `do` still runs — and calling pulumi.organization() / pulumi.stack() in
// the input file surfaces a clear "not supported" error rather than crashing or silently passing an
// empty string. Mirrors the behavior of the PCL builtin when the eval context's value is empty.
func TestDoCmdFunctionInvokeWithoutStackContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name:    tokens.PackageName("my-project"),
				Runtime: workspace.NewProjectRuntimeInfo("yaml", nil),
			}, root, nil
		},
		// Default NewF returns workspace.ErrProjectNotFound which currentStackIdentity must tolerate.
	}

	makeSpec := func() schema.PackageSpec {
		return schema.PackageSpec{
			Name: "azure",
			Functions: map[string]schema.FunctionSpec{
				"azure:index:myFunction": {
					Inputs: &schema.ObjectTypeSpec{
						Properties: map[string]schema.PropertySpec{
							"project":      {TypeSpec: schema.TypeSpec{Type: "string"}},
							"organization": {TypeSpec: schema.TypeSpec{Type: "string"}},
							"stack":        {TypeSpec: schema.TypeSpec{Type: "string"}},
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
	}

	// Sub-case 1: only project() is referenced — currentStackIdentity returns ("", "") but the
	// command should still succeed because project() doesn't depend on the stack identity.
	t.Run("project-only input succeeds", func(t *testing.T) {
		t.Parallel()
		loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
			return &testProvider{
				spec: makeSpec(),
				MockProvider: plugin.MockProvider{
					InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
						assert.Equal(t, "my-project", req.Args["project"].StringValue())
						return plugin.InvokeResponse{
							Properties: resource.PropertyMap{"ok": resource.NewProperty(true)},
						}, nil
					},
				},
			}, nil
		}

		inputFile := writeHCLFile(t, "inputs.pcl", `project = project()`)

		var stdout bytes.Buffer
		cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"azure:index:myFunction", "--input-file", inputFile})
		require.NoError(t, cmd.Execute())
	})

	// Sub-case 2: referencing organization() / stack() with no stack selected errors with the
	// "not supported" message from the PCL runtime (same behavior as `pulumi up` outside a stack).
	t.Run("organization/stack reference errors", func(t *testing.T) {
		t.Parallel()
		loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
			return &testProvider{spec: makeSpec()}, nil
		}

		inputFile := writeHCLFile(t, "inputs.pcl", `organization = organization()`)

		var stdout bytes.Buffer
		cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"azure:index:myFunction", "--input-file", inputFile})
		err := cmd.Execute()
		require.ErrorContains(t, err, "organization is not supported")
	})
}
