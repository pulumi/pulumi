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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
				"aws:s3/getAccessPoint:getAccessPoint":                       {},
				"aws:s3/getAccountPublicAccessBlock:getAccountPublicAccessBlock": {},
				"aws:ec2/getInstance:getInstance":                            {},
			},
			Resources: map[string]schema.ResourceSpec{
				"aws:s3/bucket:Bucket":     {},
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
	assert.Contains(t, output, "  aws:s3\n")
	assert.Contains(t, output, "  aws:ec2\n")
	assert.NotContains(t, output, "aws:s3/")
	assert.NotContains(t, output, "aws:ec2/")
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
