package ints

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestExitCode_StackNotFound(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Use a local file backend so we don't depend on external services.
	e.Backend = e.LocalURL()

	cmd := e.SetupCommandIn(context.Background(), e.CWD, "pulumi", "stack", "select", "does-not-exist")
	err := cmd.Run()

	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	require.Equal(t, 6, exitErr.ExitCode())
}

