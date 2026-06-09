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

package neo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUpdateID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   string
		want bool
	}{
		{"5", true},
		{"123", true},
		{"0", true},
		{"", false},
		{"7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d", false}, // preview UUID
		{"v5", false},
		{"5a", false},
	}
	for _, tt := range tests {
		assert.Equalf(t, tt.want, isUpdateID(tt.id), "isUpdateID(%q)", tt.id)
	}
}

func TestDebugSeedPromptUpdate(t *testing.T) {
	t.Parallel()

	// The seed is a short trigger line; the debugging procedure lives in the
	// pulumi-debug-failed-operation skill, which Neo's evaluator loads from this text.
	want := "Debug the failed update 5 of this stack and fix it directly in this working directory.\n"
	got := debugSeedPrompt("5")
	assert.Equal(t, want, got)

	// The procedure moved to the skill, so the seed stays a one-liner with no embedded steps.
	assert.NotContains(t, got, "<details>")
	assert.NotContains(t, got, "/api/console")
}

func TestDebugSeedPromptPreview(t *testing.T) {
	t.Parallel()

	prompt := debugSeedPrompt("7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d")
	assert.Equal(t,
		"Debug the failed preview 7f3a2b9c-1d4e-4f6a-8b2c-9e0d1a2b3c4d of this stack "+
			"and fix it directly in this working directory.\n",
		prompt)
}

func TestDebugSeedPromptNoID(t *testing.T) {
	t.Parallel()

	// With no id, the seed targets the user's most recent operation; the skill confirms which one.
	assert.Equal(t,
		"Debug my most recent Pulumi operation on this stack and fix it directly in this working directory.\n",
		debugSeedPrompt(""))
}

func TestNeoDebugCmdRegistered(t *testing.T) {
	t.Parallel()

	cmd := newNeoDebugCmd()
	assert.Equal(t, "debug [update-or-preview-id]", cmd.Use)
	require.NotNil(t, cmd.Args)
	// The id is optional (0 or 1 args); two or more is an error.
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"5"}))
	assert.Error(t, cmd.Args(cmd, []string{"5", "6"}))

	// The shared flags are present so `debug` honors the same options as `pulumi neo`.
	for _, name := range []string{"stack", "org", "cwd", "approval-mode", "permission-mode", "print"} {
		assert.NotNilf(t, cmd.Flags().Lookup(name), "flag %q", name)
	}
}
