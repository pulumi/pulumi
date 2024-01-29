package filestate

import (
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randomStackName() string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	contract.AssertNoErrorf(err, "failed to generate random stack name")
	return "test" + hex.EncodeToString(b)
}

func loginAndCreateStack(t *testing.T, cloudURL string) {
	t.Helper()

	stackName := randomStackName()
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
