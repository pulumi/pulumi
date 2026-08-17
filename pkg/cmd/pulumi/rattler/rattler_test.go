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

package rattler

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

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
			leaf("list", "ls"),
			group("webhook", leaf("new"), leaf("list")),
		),
		group("org",
			group("member", leaf("list"), leaf("remove")),
			group("webhook", leaf("new")),
		),
		group("env", leaf("list", "ls"), leaf("get"), leaf("set"), leaf("delete")),
		leaf("import"),
		&cobra.Command{Use: "secret-cmd", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)
	return root
}

func TestSuggestCommands(t *testing.T) {
	t.Parallel()

	// Each subtest builds its own tree: cobra's Commands() lazily sorts the
	// child slice in place, so sharing one tree across parallel subtests
	// races.
	find := func(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
		c, _, err := root.Find(path)
		require.NoError(t, err)
		return c
	}

	t.Run("exact leaf name beats levenshtein sibling", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(newSuggestionsTestTree(), []string{"export"})
		require.NotEmpty(t, got)
		assert.Equal(t, "pulumi stack export", got[0])
	})

	t.Run("synonyms match", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(newSuggestionsTestTree(), []string{"webhook", "create"})
		assert.Equal(t, []string{"pulumi org webhook new", "pulumi stack webhook new"}, got)
	})

	t.Run("synonym matching is symmetric", func(t *testing.T) {
		t.Parallel()
		// The synonym table maps `rm` and `delete` to the canonical `remove`,
		// and candidate names normalize through the same table as typed words,
		// so `rm` finds a command named `delete` even though neither appears
		// as the other's value in the table.
		got := suggestCommands(find(t, newSuggestionsTestTree(), "env"), []string{"rm"})
		require.NotEmpty(t, got)
		assert.Equal(t, "pulumi env delete", got[0])
	})

	t.Run("hyphen splitting and plural stemming", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(find(t, newSuggestionsTestTree(), "org"), []string{"list-members"})
		require.NotEmpty(t, got)
		assert.Equal(t, "pulumi org member list", got[0])
	})

	t.Run("subtree preferred over rest of tree", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(find(t, newSuggestionsTestTree(), "env"), []string{"lisst"})
		assert.Equal(t, []string{"pulumi env list"}, got)
	})

	t.Run("two words reach a deep command from the root", func(t *testing.T) {
		t.Parallel()
		// `stack webhook list` outscores everything: `webhook` and `list`
		// both match exactly and only `stack` goes untyped. The `webhook`
		// groups themselves also match the failing word, but land too far
		// below the winner to be offered alongside it.
		got := suggestCommands(newSuggestionsTestTree(), []string{"webhook", "list"})
		assert.Equal(t, []string{"pulumi stack webhook list"}, got)
	})

	t.Run("one word is not enough for a deep command", func(t *testing.T) {
		t.Parallel()
		// `new` matches `stack webhook new` and `org webhook new` exactly,
		// but two untyped path words each cost a point, dropping the score
		// below the cutoff: deep commands need more typed context.
		got := suggestCommands(newSuggestionsTestTree(), []string{"new"})
		assert.Empty(t, got)
	})

	t.Run("hidden commands are not suggested", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(newSuggestionsTestTree(), []string{"secret-cmd"})
		assert.Empty(t, got)
	})

	t.Run("no suggestions for gibberish", func(t *testing.T) {
		t.Parallel()
		got := suggestCommands(newSuggestionsTestTree(), []string{"xyzzyq"})
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

// The root is a group command like any other: invoked bare it prints help but
// still exits non-zero, and an unknown command fails with suggestions.
func TestRootIsAStandardGroupCommand(t *testing.T) {
	t.Parallel()

	t.Run("bare root prints help and fails", func(t *testing.T) {
		t.Parallel()
		root := newSuggestionsTestTree()
		Install(root)

		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{})

		err := root.Execute()
		require.Error(t, err)
		assert.ErrorContains(t, err, `"pulumi" requires a subcommand`)
		assert.NotEmpty(t, out.String(), "help text should be printed")
	})

	t.Run("unknown command at the root suggests across the tree", func(t *testing.T) {
		t.Parallel()
		root := newSuggestionsTestTree()
		Install(root)

		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"export"})

		err := root.Execute()
		require.Error(t, err)
		assert.ErrorContains(t, err, `unknown command "export" for "pulumi"`)
		assert.ErrorContains(t, err, "pulumi stack export")
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
	Install(root)

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

// Install's synthetic run functions only print help and fail, so commands
// that received one must be distinguishable from genuinely runnable commands
// by consumers inspecting the tree (like the CLI spec generator).
func TestHasSyntheticRun(t *testing.T) {
	t.Parallel()

	root := newSuggestionsTestTree()
	Install(root)

	find := func(path ...string) *cobra.Command {
		c := root
		for _, name := range path {
			next, _, err := c.Find([]string{name})
			require.NoError(t, err)
			c = next
		}
		return c
	}

	assert.True(t, HasSyntheticRun(root), "root")
	assert.True(t, HasSyntheticRun(find("stack")), "group command")
	assert.True(t, HasSyntheticRun(find("org", "member")), "nested group command")
	assert.False(t, HasSyntheticRun(find("import")), "runnable leaf")
	assert.False(t, HasSyntheticRun(find("stack", "export")), "nested runnable leaf")
	assert.False(t, HasSyntheticRun(nil), "nil command")
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
