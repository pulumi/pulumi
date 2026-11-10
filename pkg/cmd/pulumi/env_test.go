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

package main

import (
	"testing"

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

func TestTypedFlagEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
		path  []string
		flag  string
	}{
		{"int32", "PULUMI_OPTION_PARALLEL", "3", []string{"up"}, "parallel"},
		{"int64", "PULUMI_OPTION_LOOKBACK_DAYS", "7", []string{"org", "usage", "get"}, "lookback-days"},
		{"duration", "PULUMI_OPTION_DELAY", "2s", []string{"replay-events"}, "delay"},
		{"Delimiter", "PULUMI_OPTION_DELIMITER", ";", []string{"org", "search"}, "delimiter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)

			cmd, cleanup := NewPulumiCmd()
			defer cleanup()

			sub, _, err := cmd.Find(tt.path)
			require.NoError(t, err)

			flag := sub.Flag(tt.flag)
			require.NotNil(t, flag)
			require.Equal(t, tt.value, flag.Value.String())
		})
	}
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
