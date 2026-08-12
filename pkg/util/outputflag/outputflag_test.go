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

package outputflag

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allFormats is a fully-populated OutputFlag used as a fixture across tests.
// The R type is `string` so each renderer is a self-describing tag.
func allFormats() OutputFlag[string] {
	return OutputFlag[string]{
		RenderForTerminal: "terminal",
		RenderJSON:        "json",
		RenderMarkdown:    "markdown",
		RenderYAML:        "yaml",
		RenderCSV:         "csv",
	}
}

func newAliasCmd() (*cobra.Command, *OutputFlag[string]) {
	f := allFormats()
	cmd := &cobra.Command{
		Use:           "test",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(*cobra.Command, []string) error { return nil },
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	VarWithJSONAlias(cmd, cmd.Flags(), &f)
	return cmd, &f
}

func TestVarWithJSONAlias(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no flags", nil, "terminal"},
		{"bare --json", []string{"--json"}, "json"},
		{"-j shorthand", []string{"-j"}, "json"},
		{"--json=false stays default", []string{"--json=false"}, "terminal"},
		{"--output json", []string{"--output", "json"}, "json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd, f := newAliasCmd()
			cmd.SetArgs(tc.args)
			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.want, f.Get())
		})
	}

	// --json and --output are mutually exclusive: passing both is an error,
	// even when they agree (--json --output=json).
	conflicts := [][]string{
		{"--json", "--output", "default"},
		{"--output", "default", "--json"},
		{"--json", "--output", "json"},
		{"--json=false", "--output", "json"},
	}
	for _, args := range conflicts {
		t.Run("conflict "+strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			cmd, _ := newAliasCmd()
			cmd.SetArgs(args)
			err := cmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, "none of the others can be")
		})
	}

	t.Run("--json flag is hidden", func(t *testing.T) {
		t.Parallel()
		cmd, _ := newAliasCmd()
		require.NotNil(t, cmd.Flags().Lookup("json"))
		assert.True(t, cmd.Flags().Lookup("json").Hidden)
		require.NotNil(t, cmd.Flags().Lookup("output"))
	})
}

func TestOutputFlag_DefaultUnset(t *testing.T) {
	t.Parallel()

	f := allFormats()
	assert.Equal(t, "terminal", f.Get(), "Get() before Set should return RenderForTerminal")
	assert.Equal(t, "default", f.String(), "String() before Set should report \"default\"")
}

func TestOutputFlag_SetSelectsRenderer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", "terminal"},
		{"default", "terminal"},
		{"json", "json"},
		{"markdown", "markdown"},
		{"md", "markdown"},
		{"yaml", "yaml"},
		{"csv", "csv"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			f := allFormats()
			require.NoError(t, f.Set(tc.input))
			assert.Equal(t, tc.want, f.Get())
		})
	}
}

func TestOutputFlag_StringReflectsLastSet(t *testing.T) {
	t.Parallel()

	f := allFormats()
	require.NoError(t, f.Set("json"))
	assert.Equal(t, "json", f.String())

	require.NoError(t, f.Set("md"))
	assert.Equal(t, "markdown", f.String(), "Set normalizes the md alias to markdown")

	require.NoError(t, f.Set(""))
	assert.Equal(t, "default", f.String(), "Set(\"\") falls back to the \"default\" String()")
}

func TestOutputFlag_UnconfiguredFormatRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{"json", "json"},
		{"markdown", "markdown"},
		{"md alias", "md"},
		{"yaml", "yaml"},
		{"csv", "csv"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Empty OutputFlag has only `default` available.
			var f OutputFlag[string]
			err := f.Set(tc.input)
			require.Error(t, err)
			assert.Equal(t,
				`output "`+tc.input+`" not supported, valid values are: default`,
				err.Error())
		})
	}
}

func TestOutputFlag_UnknownFormatRejected(t *testing.T) {
	t.Parallel()

	f := allFormats()
	err := f.Set("xml")
	require.Error(t, err)
	assert.Equal(t,
		`output "xml" not supported, valid values are: default, json, markdown, yaml or csv`,
		err.Error())
}

func TestOutputFlag_PartialConfigurationListsOnlyAvailable(t *testing.T) {
	t.Parallel()

	// Only RenderJSON is configured; the error must omit markdown/yaml/csv
	// but always include "default".
	f := OutputFlag[string]{RenderJSON: "json"}
	err := f.Set("yaml")
	require.Error(t, err)
	assert.Equal(t,
		`output "yaml" not supported, valid values are: default or json`,
		err.Error())
}

func TestOutputFlag_DefaultAlwaysAcceptedEvenWhenZero(t *testing.T) {
	t.Parallel()

	// RenderForTerminal left as the zero value of R; "default" must still
	// be a legal value and Get() must return that zero value.
	var f OutputFlag[string]
	require.NoError(t, f.Set("default"))
	assert.Equal(t, "", f.Get())

	require.NoError(t, f.Set(""))
	assert.Equal(t, "", f.Get())
}

func TestOutputFlag_FailedSetLeavesPriorValue(t *testing.T) {
	t.Parallel()

	f := allFormats()
	require.NoError(t, f.Set("json"))
	require.Error(t, f.Set("xml"))

	assert.Equal(t, "json", f.Get(), "rejected Set must not overwrite the prior renderer")
	assert.Equal(t, "json", f.String(), "rejected Set must not overwrite the prior String()")
}

func TestOutputFlag_ImplementsPflagValue(t *testing.T) {
	t.Parallel()

	f := allFormats()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&f, "output", "")

	require.NoError(t, fs.Parse([]string{"--output=json"}))
	assert.Equal(t, "json", f.Get())
	assert.Equal(t, "json", f.String())

	// Reset and confirm invalid values surface through pflag too.
	f = allFormats()
	fs = pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(discard{}) // silence pflag's default usage dump on error
	fs.Var(&f, "output", "")
	err := fs.Parse([]string{"--output=xml"})
	require.Error(t, err)
	assert.ErrorContains(t, err, `output "xml" not supported`)
}

func TestOutputFlag_FunctionRenderer(t *testing.T) {
	t.Parallel()

	// Exercise the typical real-world shape: R is a renderer function.
	type renderer func() string
	f := OutputFlag[renderer]{
		RenderForTerminal: func() string { return "terminal" },
		RenderJSON:        func() string { return "json" },
	}

	assert.Equal(t, "terminal", f.Get()(), "Get() before Set should return RenderForTerminal")

	require.NoError(t, f.Set("json"))
	assert.Equal(t, "json", f.Get()())

	require.Error(t, f.Set("yaml"), "yaml must be rejected because RenderYAML is nil")
}

// discard is an io.Writer that swallows writes, used to silence pflag's
// usage output on parse errors.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
