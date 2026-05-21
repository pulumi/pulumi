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
			assert.Contains(t, output, "Usage:")
			assert.Contains(t, output, "pulumi do")
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

func TestDoCmdWithPkgArgPrintsHelp(t *testing.T) {
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

	cmd.SetArgs([]string{"aws@4.1"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Interact with aws resources and functions.

Help text about aws.

Run 'pulumi do aws@4.1 <module/resource/function> --help' for more details on usage.

Usage:
  do aws@4.1 [command]

Functions
  myFunction  Invoke the myFunction function

Resources
  myResource  Operate on the myResource resource

Modules
  myModule    Functions and resources for the myModule module

Flags:
  -h, --help                     help for aws@4.1
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")

Global Flags:
      --dry-run        Run the operation in preview mode
      --show-secrets   Show secret values in output

Use "do aws@4.1 [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())

	// Ensure that extra flags don't confuse the help message.
	stdout.Reset()
	cmd.SetArgs([]string{"--dry-run", "aws@4.1"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, expected, stdout.String())
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

Run 'pulumi do aws@4.1 <module/resource/function> --help' for more details on usage.

Usage:
  pulumi do aws@4.1 [command]

Functions
  myFunction  Invoke the myFunction function

Resources
  myResource  Operate on the myResource resource

Modules
  myModule    Functions and resources for the myModule module

Flags:
  -h, --help                     help for aws@4.1
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")

Global Flags:
      --dry-run        Run the operation in preview mode
      --show-secrets   Show secret values in output

Use "pulumi do aws@4.1 [command] --help" for more information about a command.
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

	cmd.SetArgs([]string{"aws@4.1", "myModule"})
	err := cmd.Execute()
	require.NoError(t, err)

	expected := `Functions and resources for the myModule module.

Run 'pulumi do aws@4.1 myModule <resource/function> --help' for more details on usage.

Usage:
  do aws@4.1 myModule [command]

Functions
  myOtherFunction Invoke the myOtherFunction function

Resources
  myOtherResource Operate on the myOtherResource resource

Flags:
  -h, --help   help for myModule

Global Flags:
      --dry-run                  Run the operation in preview mode
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")
      --show-secrets             Show secret values in output

Use "do aws@4.1 myModule [command] --help" for more information about a command.
`
	assert.Equal(t, expected, stdout.String())

	// Ensure that extra flags don't confuse the help message.
	stdout.Reset()
	cmd.SetArgs([]string{"--dry-run", "aws@4.1", "myModule"})
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

Run 'pulumi do pkg <module/resource/function> --help' for more details on usage.

Usage:
  do pkg [command]

Modules
  mod1             Functions and resources for the mod1 module
  mod1/mod2        Functions and resources for the mod2 module

Flags:
  -h, --help                     help for pkg
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")

Global Flags:
      --dry-run        Run the operation in preview mode
      --show-secrets   Show secret values in output

Use "do pkg [command] --help" for more information about a command.
`
	assert.Equal(t, expectedPackageHelp, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"pkg", "mod1", "--help"})
	err = cmd.Execute()
	require.NoError(t, err)

	expectedParentModuleHelp := `Functions and resources for the mod1 module.

Run 'pulumi do pkg mod1 <resource/function> --help' for more details on usage.

Usage:
  do pkg mod1 [command]

Flags:
  -h, --help   help for mod1

Global Flags:
      --dry-run                  Run the operation in preview mode
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")
      --show-secrets             Show secret values in output

Use "do pkg mod1 [command] --help" for more information about a command.
`
	assert.Equal(t, expectedParentModuleHelp, stdout.String())

	stdout.Reset()
	cmd.SetArgs([]string{"pkg", "mod1/mod2", "--help"})
	err = cmd.Execute()
	require.NoError(t, err)

	expectedNestedModuleHelp := `Functions and resources for the mod1/mod2 module.

Run 'pulumi do pkg mod1/mod2 <resource/function> --help' for more details on usage.

Usage:
  do pkg mod1/mod2 [command]

Functions
  fun         Invoke the fun function

Flags:
  -h, --help   help for mod1/mod2

Global Flags:
      --dry-run                  Run the operation in preview mode
      --provider-file string     Path to a file containing provider configuration
      --provider-format string   Format of the provider configuration file (default "pcl")
      --show-secrets             Show secret values in output

Use "do pkg mod1/mod2 [command] --help" for more information about a command.
`
	assert.Equal(t, expectedNestedModuleHelp, stdout.String())

	// module isn't runnable so help should be printed even without --help flag
	stdout.Reset()
	cmd.SetArgs([]string{"pkg", "mod1/mod2"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, expectedNestedModuleHelp, stdout.String())
}

func TestDoCmdUnknownSubcommandSuggests(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws", source)
		spec := schema.PackageSpec{
			Name: "aws",
			Resources: map[string]schema.ResourceSpec{
				"aws:s3:Bucket":   {},
				"aws:index:Stack": {},
			},
			Functions: map[string]schema.FunctionSpec{
				"aws:s3:getBucket": {},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	table := []struct {
		name           string
		args           []string
		wantMsg        string
		wantSuggestion string
		noSuggestion   bool
	}{
		{
			name:           "module subcommand wrong case",
			args:           []string{"aws", "s3", "bucket"},
			wantMsg:        `unknown command "bucket" for "do aws s3"`,
			wantSuggestion: "Bucket",
		},
		{
			name:           "module subcommand typo",
			args:           []string{"aws", "s3", "Buckt"},
			wantMsg:        `unknown command "Buckt" for "do aws s3"`,
			wantSuggestion: "Bucket",
		},
		{
			name:         "module subcommand no close match",
			args:         []string{"aws", "s3", "nothinglikethis"},
			wantMsg:      `unknown command "nothinglikethis" for "do aws s3"`,
			noSuggestion: true,
		},
		{
			name:           "package subcommand wrong case",
			args:           []string{"aws", "stack"},
			wantMsg:        `unknown command "stack" for "do aws"`,
			wantSuggestion: "Stack",
		},
		{
			name:           "resource subcommand typo",
			args:           []string{"aws", "s3", "Bucket", "creat"},
			wantMsg:        `unknown command "creat" for "do aws s3 Bucket"`,
			wantSuggestion: "create",
		},
		{
			name:           "function name wrong case",
			args:           []string{"aws", "s3", "getbucket"},
			wantMsg:        `unknown command "getbucket" for "do aws s3"`,
			wantSuggestion: "getBucket",
		},
		{
			name:           "typo'd subcommand followed by leaf flag",
			args:           []string{"aws", "s3", "getbucket", "--input-file", "./inputs.pcl"},
			wantMsg:        `unknown command "getbucket" for "do aws s3"`,
			wantSuggestion: "getBucket",
		},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
			if tc.noSuggestion {
				assert.NotContains(t, err.Error(), "Did you mean")
			} else {
				assert.Contains(t, err.Error(), "Did you mean this?")
				assert.Contains(t, err.Error(), tc.wantSuggestion)
			}
		})
	}
}

// TestDoCmdLeafFlagValidationStrict validates that container commands (package, module, resource) tolerate unknown
// flags so that typo'd subcommand names produce a "did you mean" hint instead of a confusing "unknown flag" error. Leaf
// commands (functions, create/read/patch/delete/list) must still strictly reject unknown flags.
func TestDoCmdLeafFlagValidationStrict(t *testing.T) {
	t.Parallel()

	mlm := &cmdBackend.MockLoginManager{}
	mws := &pkgWorkspace.MockContext{}
	loader := func(ctx context.Context, pctx *plugin.Context, wd, source string) (plugin.Provider, error) {
		assert.Equal(t, "aws", source)
		spec := schema.PackageSpec{
			Name: "aws",
			Resources: map[string]schema.ResourceSpec{
				"aws:s3:Bucket": {},
			},
			Functions: map[string]schema.FunctionSpec{
				"aws:s3:getBucket": {},
			},
		}
		return &testProvider{spec: spec}, nil
	}

	table := []struct {
		name        string
		args        []string
		wantUnknown bool
	}{
		{
			name:        "function leaf unknown flag",
			args:        []string{"aws", "s3", "getBucket", "--input-files", "/file"},
			wantUnknown: true,
		},
		{
			name:        "function leaf valid flag",
			args:        []string{"aws", "s3", "getBucket", "--input-file", "/file"},
			wantUnknown: false,
		},
		{
			name:        "resource verb leaf unknown flag",
			args:        []string{"aws", "s3", "Bucket", "read", "abc", "--no-such-flag"},
			wantUnknown: true,
		},
		{
			name:        "resource verb leaf valid args",
			args:        []string{"aws", "s3", "Bucket", "read", "abc"},
			wantUnknown: false,
		},
	}

	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.wantUnknown {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown flag")
			} else {
				// We don't care whether the command succeeds or fails downstream (the MockProvider
				// returns "not implemented" for Read/Invoke); we just need to prove that the flag
				// itself parsed cleanly and didn't trip the strict leaf parser.
				if err != nil {
					assert.NotContains(t, err.Error(), "unknown flag")
				}
			}
		})
	}
}
