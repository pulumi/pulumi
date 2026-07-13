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

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
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
			wantSuggestions: []string{"pulumi env ls"},
		},
		{
			// `stack` is runnable and has subcommands, so this exercises the
			// argument-specification path rather than the group-command path.
			args:            []string{"stack", "expor"},
			wantErr:         `unknown command "expor" for "pulumi stack"`,
			wantSuggestions: []string{"pulumi stack export"},
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
		{args: []string{"env", "list"}, wantName: "ls"},
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

// A bare `pulumi` must keep printing help and exiting zero.
//
//nolint:paralleltest // NewPulumiCmd registers env vars in a process-wide registry
func TestBareRootPrintsHelp(t *testing.T) {
	pulumiCmd, cleanup := NewPulumiCmd()
	defer cleanup()

	var stdout, stderr bytes.Buffer
	pulumiCmd.SetOut(&stdout)
	pulumiCmd.SetErr(&stderr)
	pulumiCmd.SetArgs([]string{})

	err := pulumiCmd.Execute()
	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String(), "help text should be printed")
}

// newSuggestionsTestTree builds a small synthetic command tree mirroring the
// shapes that matter for suggestion ranking.
func newSuggestionsTestTree() *cobra.Command {
	leaf := func(use string, aliases ...string) *cobra.Command {
		return &cobra.Command{Use: use, Aliases: aliases, Run: func(*cobra.Command, []string) {}}
	}
	group := func(use string, children ...*cobra.Command) *cobra.Command {
		c := &cobra.Command{Use: use}
		c.AddCommand(children...)
		return c
	}

	root := group("pulumi",
		group("stack",
			leaf("export"),
			leaf("import"),
			leaf("ls", "list"),
			group("webhook", leaf("new"), leaf("list")),
		),
		group("org",
			group("member", leaf("list"), leaf("remove")),
			group("webhook", leaf("new")),
		),
		group("env", leaf("ls", "list"), leaf("get"), leaf("set")),
		leaf("import"),
		&cobra.Command{Use: "secret-cmd", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)
	return root
}

func TestSuggestCommands(t *testing.T) {
	t.Parallel()

	root := newSuggestionsTestTree()
	find := func(path ...string) *cobra.Command {
		c, _, err := root.Find(path)
		require.NoError(t, err)
		return c
	}

	t.Run("exact leaf name beats levenshtein sibling", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(root, []string{"export"})
		require.NotEmpty(t, got)
		assert.Equal(t, "pulumi stack export", got[0])
	})

	t.Run("synonyms match", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(root, []string{"webhook", "create"})
		assert.Equal(t, []string{"pulumi org webhook new", "pulumi stack webhook new"}, got)
	})

	t.Run("hyphen splitting and plural stemming", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(find("org"), []string{"list-members"})
		require.NotEmpty(t, got)
		assert.Equal(t, "pulumi org member list", got[0])
	})

	t.Run("subtree preferred over rest of tree", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(find("env"), []string{"lisst"})
		assert.Equal(t, []string{"pulumi env ls"}, got)
	})

	t.Run("hidden commands are not suggested", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(root, []string{"secret-cmd"})
		assert.Empty(t, got)
	})

	t.Run("no suggestions for gibberish", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(root, []string{"xyzzyq"})
		assert.Empty(t, got)
	})

	t.Run("at most three suggestions", func(t *testing.T) {
		t.Parallel()
		crowded := &cobra.Command{Use: "pulumi"}
		parent := &cobra.Command{Use: "things"}
		for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
			parent.AddCommand(&cobra.Command{Use: name, Run: func(*cobra.Command, []string) {}})
		}
		crowded.AddCommand(parent)
		got := suggestCommands(crowded, []string{"things"})
		assert.LessOrEqual(t, len(got), maxSuggestions)
	})
}

// A runnable command with subcommands may accept legitimate positional args.
// Only the args past its argument specification can be an attempted
// subcommand, and the error must blame the first of those, not the first arg
// outright.
func TestRunnableParentBlamesArgPastSpec(t *testing.T) {
	t.Parallel()

	parent := &cobra.Command{Use: "thing", RunE: func(*cobra.Command, []string) error { return nil }}
	constrictor.AttachArguments(parent, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "name"}},
	})
	parent.AddCommand(&cobra.Command{Use: "export", Run: func(*cobra.Command, []string) {}})
	root := &cobra.Command{Use: "pulumi"}
	root.AddCommand(parent)
	installUnknownCommandHandling(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	root.SetArgs([]string{"thing", "myname", "expor"})
	err := root.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown command "expor" for "pulumi thing"`)
	assert.ErrorContains(t, err, "pulumi thing export")
	assert.NotContains(t, err.Error(), "myname")

	// Args within the specification alone must still reach the command.
	root.SetArgs([]string{"thing", "myname"})
	require.NoError(t, root.Execute())
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		token string
		want  []string
	}{
		{"list-members", []string{"list", "member"}},
		{"ls", []string{"list"}},
		{"create", []string{"new"}},
		{"audit-log", []string{"audit", "log"}},
		{"creates", []string{"new"}},
		{"Webhook", []string{"webhook"}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, normalize(c.token), "normalize(%q)", c.token)
	}
}
