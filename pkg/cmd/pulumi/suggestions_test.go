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

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
)

// An integration check that rattler.Install is wired into NewPulumiCmd: an
// unknown command on the real tree fails non-zero with a whole-tree
// suggestion. Suggestion ranking itself is covered by the rattler package's
// own tests. The error surfaces from args validation, before any of the
// root's persistent init hooks run.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestUnknownCommandSuggestions(t *testing.T) {
	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{"export"})

	err := pulumiCmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown command "export" for "pulumi"`)
	assert.ErrorContains(t, err, "Did you mean this?")
	assert.ErrorContains(t, err, "pulumi stack export")
	assert.Equal(t, cmd.ExitCodeError, cmd.ExitCodeFor(err))
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
