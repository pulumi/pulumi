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
	"io"
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

func panicLoader(context.Context, diag.Sink, string, string) (io.Closer, plugin.Provider, error) {
	panic("not implemented")
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
			cmd := NewDoCmd(mlm, mws, panicLoader)
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

type nopCloser struct {
	closed bool
}

func (nc *nopCloser) Close() error {
	nc.closed = true
	return nil
}

func closer(t *testing.T) io.Closer {
	c := &nopCloser{}
	// Always assert that the plugin context is closed by the end of the test.
	t.Cleanup(func() {
		assert.True(t, c.closed, "expected closer to be closed")
	})
	return c
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

func (p *testProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	schemaBytes, err := json.Marshal(p.spec)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	schema := string(schemaBytes)
	return plugin.GetSchemaResponse{Schema: []byte(schema)}, nil
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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
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
  -h, --help                   help for aws@4.1
      --provider-file string   Path to a file containing provider configuration

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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{spec: spec}, nil
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
	rootCmd.AddCommand(NewDoCmd(mlm, mws, loader))

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
  -h, --help                   help for aws@4.1
      --provider-file string   Path to a file containing provider configuration

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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
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
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
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
      --dry-run                Run the operation in preview mode
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output

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
	loader := func(ctx context.Context, sink diag.Sink, wd, source string) (io.Closer, plugin.Provider, error) {
		assert.Equal(t, "pkg", source)
		spec := schema.PackageSpec{
			Name: "pkg",
			Functions: map[string]schema.FunctionSpec{
				"pkg:mod1/mod2:fun": {},
			},
		}
		return closer(t), &testProvider{spec: spec}, nil
	}

	var stdout bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader)
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
  -h, --help                   help for pkg
      --provider-file string   Path to a file containing provider configuration

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
      --dry-run                Run the operation in preview mode
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output

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
      --dry-run                Run the operation in preview mode
      --provider-file string   Path to a file containing provider configuration
      --show-secrets           Show secret values in output

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
