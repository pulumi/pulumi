package diy

import (
	"os/exec"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loginAndCreateStack(t *testing.T, cloudURL string) {
	t.Helper()

	stackName := ptesting.RandomStackName()
	out, err := exec.Command("pulumi", "login", cloudURL).CombinedOutput()
	require.NoError(t, err, string(out))

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
	out, err = exec.Command("pulumi", "stack", "init", stackName).CombinedOutput()
	require.NoError(t, err, string(out))
	defer func() {
		out, err := exec.Command("pulumi", "stack", "rm", "--yes", "-s", stackName).CombinedOutput()
		assert.NoError(t, err, string(out))
	}()

	out, err = exec.Command("pulumi", "stack", "select", stackName).CombinedOutput()
	require.NoError(t, err, string(out))

	out, err = exec.Command("pulumi", "stack", "ls").CombinedOutput()
	assert.NoError(t, err, string(out))
	assert.Contains(t, string(out), stackName+"*")
}
