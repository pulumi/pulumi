package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
				UpgradeF: func(context.Context, *filestate.UpgradeOptions) error {
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
				UpgradeF: func(context.Context, *filestate.UpgradeOptions) error {
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

//nolint:paralleltest // subtests have shared state
func TestStateUpgradeProjectNameWidget(t *testing.T) {
	t.Parallel()

	// Checks the behavior of the prompt for project names
	// when they're missing.

	if runtime.GOOS == "windows" {
		t.Skip("Skipping: Cannot create pseudo-terminal on Windows")
	}

	// This is difficult to test because of how terminal-based this is.
	// To test this:
	//
	// - We set up a pseduo-terminal (with the pty package).
	//   This will tell survey that it's running in an interactive terminal.
	// - We connect that to the expect package,
	//   which lets us simulate user input and read the output.
	// - Lastly, expect doesn't actually interpret terminal escape sequences,
	//   so we pass the output of survey through a vt100 terminal emulator
	//   (with the vt10x package), allowing expect to operate on plain text.

	ptty, tty, err := pty.Open()
	require.NoError(t, err, "creating pseudo-terminal")

	console, err := expect.NewConsole(
		expect.WithStdin(ptty),
		expect.WithStdout(
			vt10x.New(vt10x.WithWriter(tty)),
			// Also write to the test log
			// so that if this test fails,
			// we can see what the user would have seen.
			iotest.LogWriterPrefixed(t, "[stdout] "),
		),
		expect.WithCloser(ptty, tty),
		// Without this timeout, the test will hang forever
		// if expectations don't match.
		expect.WithDefaultTimeout(time.Second),
	)
	require.NoError(t, err, "creating console")
	defer func() {
		assert.NoError(t, console.Close(), "close console")
	}()

	expect := func(t *testing.T, s string) {
		t.Helper()

		t.Logf("expect(%q)", s)
		_, err := console.ExpectString(s)
		require.NoError(t, err)
	}

	sendLine := func(t *testing.T, s string) {
		t.Helper()

		t.Logf("send(%q)", s)
		_, err := console.SendLine(s)
		require.NoError(t, err)
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)

		stacks := []tokens.StackName{
			tokens.MustParseStackName("foo"),
			tokens.MustParseStackName("bar"),
			tokens.MustParseStackName("baz"),
		}
		projects := make([]tokens.Name, len(stacks))

		err := (&stateUpgradeProjectNameWidget{
			Stdin:  console.Tty(),
			Stdout: console.Tty(),
			Stderr: iotest.LogWriterPrefixed(t, "[stderr] "),
		}).Prompt(stacks, projects)
		assert.NoError(t, err, "prompt failed")
		assert.Equal(t, []tokens.Name{"foo-project", "", "baz-project"}, projects)

		// We need to close the TTY after we're done here
		// so that ExpectEOF unblocks.
		assert.NoError(t, console.Tty().Close(), "close tty")
	}()
	defer func() {
		select {
		case <-donec:
			// Goroutine exited normally.

		case <-time.After(time.Second):
			t.Error("timed out waiting for test to finish")
		}
	}()

	expect(t, "Found stacks without a project name")

	// Subtests must be run serially, in-order
	// because they share the same console.

	t.Run("valid name", func(t *testing.T) {
		expect(t, "Stack foo")
		sendLine(t, "foo-project")
	})

	t.Run("bad name", func(t *testing.T) {
		expect(t, "Stack bar")
		sendLine(t, "not a valid project name")
		expect(t, "project names may only contain alphanumerics")
	})

	t.Run("skip", func(t *testing.T) {
		expect(t, "Stack bar")
		sendLine(t, "")
	})

	t.Run("long name", func(t *testing.T) {
		expect(t, "Stack baz")
		sendLine(t, strings.Repeat("a", 101)) // max length is 100
		expect(t, "project names are limited to 100 characters")
	})

	t.Run("recovery after bad name", func(t *testing.T) {
		expect(t, "Stack baz")
		sendLine(t, "baz-project")
	})

	// ExpectEOF blocks until the console reaches EOF on its input.
	// This will happen when the widget exits and closes the TTY.
	_, err = console.ExpectEOF()
	assert.NoError(t, err, "expect EOF")
}

func TestStateUpgradeProjectNameWidget_noStacks(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping: Cannot create pseudo-terminal on Windows")
	}

	ptty, tty, err := pty.Open()
	require.NoError(t, err, "creating pseudo-terminal")
	defer func() {
		assert.NoError(t, ptty.Close())
		assert.NoError(t, tty.Close())
	}()

	err = (&stateUpgradeProjectNameWidget{
		Stdin:  tty,
		Stdout: tty,
		Stderr: iotest.LogWriterPrefixed(t, "[stderr] "),
	}).Prompt([]tokens.StackName{}, []tokens.Name{})
	require.NoError(t, err)
}

func TestStateUpgradeProjectNameWidget_notATerminal(t *testing.T) {
	t.Parallel()

	stacks := []tokens.StackName{
		tokens.MustParseStackName("foo"),
		tokens.MustParseStackName("bar"),
		tokens.MustParseStackName("baz"),
	}
	projects := make([]tokens.Name, len(stacks))

	err := (&stateUpgradeProjectNameWidget{
		Stdin:  bytes.NewReader(nil),
		Stdout: bytes.NewBuffer(nil),
		Stderr: iotest.LogWriterPrefixed(t, "[stderr] "),
	}).Prompt(stacks, projects)
	require.NoError(t, err)

	// No change expected.
	assert.Equal(t, []tokens.Name{"", "", ""}, projects)
}

type stubFileBackend struct {
	filestate.Backend

	UpgradeF func(context.Context, *filestate.UpgradeOptions) error
}

var _ filestate.Backend = (*stubFileBackend)(nil)

func (f *stubFileBackend) Upgrade(ctx context.Context, opts *filestate.UpgradeOptions) error {
	return f.UpgradeF(ctx, opts)
}
