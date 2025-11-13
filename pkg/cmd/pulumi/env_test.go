// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
