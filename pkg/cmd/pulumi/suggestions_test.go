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

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unknown commands must be met with suggestions drawn from the whole command
// tree, not just the failing command's siblings. These run against the real
// command tree; the errors surface from args validation, before any of the
// root's persistent init hooks run.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestUnknownCommandSuggestions(t *testing.T) {
	cases := []struct {
		args            []string
		wantErr         string
		wantSuggestions []string
	}{
		{
			args:            []string{"export"},
			wantErr:         `unknown command "export" for "pulumi"`,
			wantSuggestions: []string{"pulumi stack export"},
		},
		{
			args:            []string{"org", "list-members"},
			wantErr:         `unknown command "list-members" for "pulumi org"`,
			wantSuggestions: []string{"pulumi org member list"},
		},
		{
			args:            []string{"webhook", "create"},
			wantErr:         `unknown command "webhook" for "pulumi"`,
			wantSuggestions: []string{"pulumi stack webhook new", "pulumi org webhook new"},
		},
		{
			args:            []string{"env", "lisst"},
			wantErr:         `unknown command "lisst" for "pulumi env"`,
			wantSuggestions: []string{"pulumi env list"},
		},
		{
			// `stack` is runnable and has subcommands, so this exercises the
			// argument-specification path rather than the group-command path.
			args:            []string{"stack", "expor"},
			wantErr:         `unknown command "expor" for "pulumi stack"`,
			wantSuggestions: []string{"pulumi stack export"},
		},
		{
			// Four levels deep: the resolved path words all count toward the
			// score, so the sibling under the same deep group wins.
			args:            []string{"stack", "webhook", "delivery", "lst"},
			wantErr:         `unknown command "lst" for "pulumi stack webhook delivery"`,
			wantSuggestions: []string{"pulumi stack webhook delivery list"},
		},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			pulumiCmd, cleanup := NewPulumiCmd()
			defer cleanup()

			var stdout, stderr bytes.Buffer
			pulumiCmd.SetOut(&stdout)
			pulumiCmd.SetErr(&stderr)
			pulumiCmd.SetArgs(c.args)

			err := pulumiCmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, c.wantErr)
			assert.ErrorContains(t, err, "Did you mean this?")
			for _, s := range c.wantSuggestions {
				assert.ErrorContains(t, err, s)
			}
		})
	}
}

// Common synonyms should resolve as aliases rather than fail with a
// suggestion.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestListAliases(t *testing.T) {
	cases := []struct {
		args     []string
		wantName string
	}{
		{args: []string{"env", "ls"}, wantName: "list"},
		{args: []string{"org", "member", "ls"}, wantName: "list"},
		{args: []string{"stack", "webhook", "ls"}, wantName: "list"},
	}

	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			pulumiCmd, cleanup := NewPulumiCmd()
			defer cleanup()

			found, _, err := pulumiCmd.Find(c.args)
			require.NoError(t, err)
			assert.Equal(t, c.wantName, found.Name())
		})
	}
}

// A bare `pulumi` is a group command like any other: it prints help but exits
// non-zero.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestBareRootPrintsHelpAndFails(t *testing.T) {
	// A bare invocation reaches the root's RunE, so the persistent init hooks
	// run; keep the async update check from hitting the network.
	t.Setenv("PULUMI_SKIP_UPDATE_CHECK", "true")

	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{})

	err := pulumiCmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, `"pulumi" requires a subcommand`)
	assert.NotEmpty(t, stdout.String(), "help text should be printed")
}
