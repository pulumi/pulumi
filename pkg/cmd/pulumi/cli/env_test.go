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

package cli

import (
	"log"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/stretchr/testify/require"
)

func TestDeclaredEnvironmentVariable(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	refresh, err := up.PersistentFlags().GetString("refresh")
	require.NoError(t, err)
	require.Equal(t, "true", refresh)
}

func TestEnvironmentVariableWorksWithMultipleCommands(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	preview, _, err := cmd.Find([]string{"preview"})
	require.NoError(t, err)

	refresh, err := preview.PersistentFlags().GetString("refresh")
	require.NoError(t, err)
	require.Equal(t, "true", refresh)
}

func TestThatDefaultsAreNotOverriddenByEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	color, err := cmd.PersistentFlags().GetString("color")

	require.NoError(t, err)
	require.Equal(t, "auto", color)
}

func TestBooleanEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_EMOJI", "true")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	emoji, err := cmd.PersistentFlags().GetBool("emoji")
	require.NoError(t, err)
	require.True(t, emoji)
}

func TestNumericEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_VERBOSE", "2")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	verbose, err := cmd.PersistentFlags().GetInt("verbose")
	require.NoError(t, err)
	require.Equal(t, 2, verbose)
}

func TestStringArrayEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_TARGET", "foo,bar,baz")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	targets, err := up.PersistentFlags().GetStringArray("target")
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, targets)
}

func TestStringArrayOneElement(t *testing.T) {
	t.Setenv("PULUMI_OPTION_TARGET", "foo")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	targets, err := up.PersistentFlags().GetStringArray("target")
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, targets)
}

func TestStringSliceEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_MAPPINGS", "foo,bar,baz")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	convert, _, err := cmd.Find([]string{"convert"})
	require.NoError(t, err)

	mappings, err := convert.PersistentFlags().GetStringSlice("mappings")
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, mappings)
}

func TestStringArrayNoSplitOnComma(t *testing.T) {
	t.Setenv("PULUMI_OPTION_PLAINTEXT", "\"a=foo,bar,baz\"")

	cmd, cleanup := NewPulumiCmd()
	defer cleanup()

	config, _, err := cmd.Find([]string{"config", "set-all"})
	require.NoError(t, err)

	plaintext, err := config.PersistentFlags().GetStringArray("plaintext")
	require.NoError(t, err)
	require.Equal(t, []string{"a=foo,bar,baz"}, plaintext)
}

func TestEmbeddedCommandRestoresProcessFlags(t *testing.T) {
	t.Setenv("PULUMI_OPTION_NON_INTERACTIVE", "true")
	oldEmoji, oldQualified := cmdutil.Emoji, cmdutil.FullyQualifyStackNames
	oldInteractive, oldIntegrity := cmdutil.DisableInteractive, backend.DisableIntegrityChecking
	writer := log.Writer()
	called := false
	cmd, cleanup := NewPulumiCmd(Options{Embedded: true, AICommands: []*cobra.Command{{
		Use: "embedded-test",
		RunE: func(cmd *cobra.Command, args []string) error {
			called = true
			require.True(t, cmdutil.DisableInteractive)
			require.Equal(t, writer, log.Writer())
			return nil
		},
	}}})
	defer cleanup()
	cmd.SetArgs([]string{"embedded-test"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))
	require.True(t, called)
	cleanup()
	require.Equal(t, oldEmoji, cmdutil.Emoji)
	require.Equal(t, oldQualified, cmdutil.FullyQualifyStackNames)
	require.Equal(t, oldInteractive, cmdutil.DisableInteractive)
	require.Equal(t, oldIntegrity, backend.DisableIntegrityChecking)
	require.Equal(t, writer, log.Writer())
}

func TestRepeatedCommandsDoNotGrowEnvironmentRegistry(t *testing.T) {
	_, cleanup := NewPulumiCmd(Options{Embedded: true})
	cleanup()
	count := len(declared.Variables())
	t.Setenv("PULUMI_OPTION_RUN_PROGRAM", "true")
	command, cleanup := NewPulumiCmd(Options{Embedded: true})
	defer cleanup()
	require.Len(t, declared.Variables(), count)
	preview, _, err := command.Find([]string{"preview"})
	require.NoError(t, err)
	value, err := preview.PersistentFlags().GetBool("run-program")
	require.NoError(t, err)
	require.True(t, value)
}

func TestInt32EnvironmentVariable(t *testing.T) {
	t.Setenv("PULUMI_OPTION_PARALLEL", "7")
	command, cleanup := NewPulumiCmd(Options{Embedded: true})
	defer cleanup()
	preview, _, err := command.Find([]string{"preview"})
	require.NoError(t, err)
	value, err := preview.PersistentFlags().GetInt32("parallel")
	require.NoError(t, err)
	require.Equal(t, int32(7), value)
}

func TestEmbeddedCommandScalarEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_COUNT", "7")
	t.Setenv("PULUMI_OPTION_GRANULARITY", "2s")
	command, cleanup := NewPulumiCmd(Options{Embedded: true})
	defer cleanup()
	var check func(*cobra.Command)
	count, duration := false, false
	check = func(cmd *cobra.Command) {
		visit := func(flag *pflag.Flag) {
			switch flag.Name {
			case "count":
				if flag.Value.Type() == "int64" {
					require.Equal(t, "7", flag.Value.String())
					count = true
				}
			case "granularity":
				require.Equal(t, "2s", flag.Value.String())
				duration = true
			}
		}
		cmd.Flags().VisitAll(visit)
		cmd.PersistentFlags().VisitAll(visit)
		for _, child := range cmd.Commands() {
			check(child)
		}
	}
	check(command)
	require.True(t, count)
	require.True(t, duration)
}
