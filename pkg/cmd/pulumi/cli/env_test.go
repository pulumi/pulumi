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
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func TestDeclaredEnvironmentVariable(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	refresh, err := up.PersistentFlags().GetString("refresh")
	require.NoError(t, err)
	require.Equal(t, "true", refresh)
}

func TestEnvironmentVariableWorksWithMultipleCommands(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	preview, _, err := cmd.Find([]string{"preview"})
	require.NoError(t, err)

	refresh, err := preview.PersistentFlags().GetString("refresh")
	require.NoError(t, err)
	require.Equal(t, "true", refresh)
}

func TestThatDefaultsAreNotOverriddenByEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_REFRESH", "true")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	color, err := cmd.PersistentFlags().GetString("color")

	require.NoError(t, err)
	require.Equal(t, "auto", color)
}

func TestBooleanEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_EMOJI", "true")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	emoji, err := cmd.PersistentFlags().GetBool("emoji")
	require.NoError(t, err)
	require.True(t, emoji)
}

func TestNumericEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_VERBOSE", "2")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	verbose, err := cmd.PersistentFlags().GetInt("verbose")
	require.NoError(t, err)
	require.Equal(t, 2, verbose)
}

func TestStringArrayEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_TARGET", "foo,bar,baz")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	targets, err := up.PersistentFlags().GetStringArray("target")
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, targets)
}

func TestStringArrayOneElement(t *testing.T) {
	t.Setenv("PULUMI_OPTION_TARGET", "foo")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	up, _, err := cmd.Find([]string{"up"})
	require.NoError(t, err)

	targets, err := up.PersistentFlags().GetStringArray("target")
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, targets)
}

func TestStringSliceEnvironmentVariables(t *testing.T) {
	t.Setenv("PULUMI_OPTION_MAPPINGS", "foo,bar,baz")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	convert, _, err := cmd.Find([]string{"convert"})
	require.NoError(t, err)

	mappings, err := convert.PersistentFlags().GetStringSlice("mappings")
	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, mappings)
}

func TestStringArrayNoSplitOnComma(t *testing.T) {
	t.Setenv("PULUMI_OPTION_PLAINTEXT", "\"a=foo,bar,baz\"")

	cmd, cleanup := NewPulumiCmd(Options{})
	defer cleanup()

	config, _, err := cmd.Find([]string{"config", "set-all"})
	require.NoError(t, err)

	plaintext, err := config.PersistentFlags().GetStringArray("plaintext")
	require.NoError(t, err)
	require.Equal(t, []string{"a=foo,bar,baz"}, plaintext)
}

func TestNewPulumiCmdLeavesProcessStateUnchanged(t *testing.T) {
	t.Setenv("PULUMI_OPTION_EMOJI", "false")
	t.Setenv("PULUMI_OPTION_NON_INTERACTIVE", "true")
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")
	defer func(emoji bool, interactive bool, qualify bool, integrity bool) {
		cmdutil.Emoji = emoji
		cmdutil.DisableInteractive = interactive
		cmdutil.FullyQualifyStackNames = qualify
		backend.DisableIntegrityChecking = integrity
	}(cmdutil.Emoji, cmdutil.DisableInteractive, cmdutil.FullyQualifyStackNames, backend.DisableIntegrityChecking)
	cmdutil.Emoji = true
	cmdutil.DisableInteractive = false
	cmdutil.FullyQualifyStackNames = false
	backend.DisableIntegrityChecking = false
	// A host root command has already set up the process.
	processSetup.Do(func() {})

	_, cleanup := NewPulumiCmd(Options{})
	defer cleanup()
	declaredCount := len(declared.Variables())
	embedded, cleanup2 := NewPulumiCmd(Options{})
	defer cleanup2()
	embedded.SetArgs([]string{"--fully-qualify-stack-names", "--disable-integrity-checking", "version"})
	embedded.SetOut(io.Discard)
	require.NoError(t, embedded.Execute())

	require.True(t, cmdutil.Emoji)
	require.False(t, cmdutil.DisableInteractive)
	require.False(t, cmdutil.FullyQualifyStackNames)
	require.False(t, backend.DisableIntegrityChecking)
	require.Len(t, declared.Variables(), declaredCount)
}
