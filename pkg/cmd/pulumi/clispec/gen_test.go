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

package clispec

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExcludeMenuWithoutPropagateKeepsSubcommands(t *testing.T) {
	t.Parallel()

	noop := func(*cobra.Command, []string) {}
	root := &cobra.Command{Use: "pulumi"}
	logs := &cobra.Command{Use: "logs", Run: noop}
	logs.AddCommand(&cobra.Command{Use: "list", Run: noop})
	root.AddCommand(logs)

	build := func(propagate bool) any {
		overrides := &Overrides{Scopes: []OverridesScope{{Path: []string{"logs"}, Propagate: propagate, Exclude: true}}}
		return buildStructure(logs, overrides, []string{"logs"}, nil)
	}

	menu, ok := build(false).(Menu)
	require.True(t, ok)
	assert.False(t, menu.Executable)
	assert.Contains(t, menu.Commands, "list")

	assert.Nil(t, build(true))
}
