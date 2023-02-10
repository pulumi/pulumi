package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

func TestStateUpgradeCommand_Run_upgrade(t *testing.T) {
	t.Parallel()

	var called bool
	cmd := stateUpgradeCmd{
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &stubFileBackend{
				UpgradeF: func(context.Context) error {
					called = true
					return nil
				},
			}, nil
		},
		Stdin:  strings.NewReader("yes\n"),
		Stdout: io.Discard,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.True(t, called, "Upgrade was never called")
}

func TestStateUpgradeCommand_Run_upgradeRejected(t *testing.T) {
	t.Parallel()

	cmd := stateUpgradeCmd{
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &stubFileBackend{
				UpgradeF: func(context.Context) error {
					t.Fatal("Upgrade should not be called")
					return nil
				},
			}, nil
		},
		Stdin:  strings.NewReader("no\n"),
		Stdout: io.Discard,
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)
}

func TestStateUpgradeCommand_Run_unsupportedBackend(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := stateUpgradeCmd{
		Stdout: &stdout,
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{}, nil
		},
	}

	// Non-filestate backend is already up-to-date.
	err := cmd.Run(context.Background())
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Nothing to do")
}

func TestStateUpgradeCmd_Run_backendError(t *testing.T) {
	t.Parallel()

	giveErr := errors.New("great sadness")
	cmd := stateUpgradeCmd{
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return nil, giveErr
		},
	}

	err := cmd.Run(context.Background())
	assert.ErrorIs(t, err, giveErr)
}

type stubFileBackend struct {
	filestate.Backend

	UpgradeF func(context.Context) error
}

func (f *stubFileBackend) Upgrade(ctx context.Context) error {
	return f.UpgradeF(ctx)
}
