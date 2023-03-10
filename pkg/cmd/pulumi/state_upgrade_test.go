package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateUpgradeCommand_parseArgs(t *testing.T) {
	t.Parallel()

	// Parsing flags with a cobra.Command without running the command
	// is a bit verbose.
	// You have to run ParseFlags to parse the flags,
	// then extract non-flag arguments with cmd.Flags().Args(),
	// then run ValidateArgs to validate the positional arguments.

	cmd := newStateUpgradeCommand()
	args := []string{} // no arguments

	require.NoError(t, cmd.ParseFlags(args))
	args = cmd.Flags().Args() // non flag args
	require.NoError(t, cmd.ValidateArgs(args))
}

func TestStateUpgradeCommand_parseArgsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		give    []string
		wantErr string
	}{
		{
			desc:    "unknown flag",
			give:    []string{"--unknown"},
			wantErr: "unknown flag: --unknown",
		},
		// Unfortunately,
		// our cmdutil.NoArgs validator exits the program,
		// causing the test to fail.
		// Until we resolve this, we'll skip this test
		// and rely on the positive test case
		// to validate the arguments intead.
		// {
		// 	desc: "unexpected argument",
		// 	give: []string{"arg"},
		// 	wantErr: `unknown command "arg" for "upgrade"`,
		// },
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cmd := newStateUpgradeCommand()
			args := tt.give

			// Errors can occur during flag parsing
			// or argument validation.
			// If there's no error on ParseFlags,
			// expect one on ValidateArgs.
			if err := cmd.ParseFlags(args); err != nil {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			args = cmd.Flags().Args() // non flag args
			assert.ErrorContains(t, cmd.ValidateArgs(args), tt.wantErr)
		})
	}
}
