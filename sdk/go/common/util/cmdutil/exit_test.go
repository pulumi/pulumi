package cmdutil

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunFunc_Bail(t *testing.T) {
	t.Parallel()

	// Verifies that a use of RunFunc that returns BailError
	// will cause the program to exit with a non-zero exit code
	// without printing an error message.
	//
	// Unfortunately, we can't test this directly,
	// because the `os.Exit` call in RunResultFunc.
	//
	// Instead, we'll re-run the test binary,
	// and have it run TestFakeCommand.
	// We'll verify the output of that binary instead.

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run=^TestFakeCommand$")
	cmd.Env = append(os.Environ(), "TEST_FAKE=1")

	// Write output to the buffer and to the test logger.
	var buff bytes.Buffer
	output := io.MultiWriter(&buff, iotest.LogWriter(t))
	cmd.Stdout = output
	cmd.Stderr = output

	err = cmd.Run()
	require.Error(t, err)
	if exitErr := new(exec.ExitError); assert.ErrorAs(t, err, &exitErr) {
		assert.NotZero(t, exitErr.ExitCode())
	}

	assert.Empty(t, buff.String())
}

//nolint:paralleltest // not a real test
func TestFakeCommand(t *testing.T) {
	if os.Getenv("TEST_FAKE") != "1" {
		// This is not a real test.
		// It's a fake test that we'll run as a subprocess
		// to verify that the RunFunc function works correctly.
		// See TestRunFunc_Bail for more details.
		return
	}

	cmd := &cobra.Command{
		Run: RunFunc(func(cmd *cobra.Command, args []string) error {
			return result.BailErrorf("bail")
		}),
	}
	err := cmd.Execute()
	// Unreachable: RunFunc should have called os.Exit.
	assert.Fail(t, "unreachable", "RunFunc should have called os.Exit: %v", err)
}
