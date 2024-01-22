package filestate

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loginAndCreateStack(t *testing.T, cloudURL string) {
	t.Helper()
	out, err := exec.Command("pulumi", "login", cloudURL).CombinedOutput()
	require.NoError(t, err, string(out))

	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")
	out, err = exec.Command("pulumi", "stack", "init", "testing").CombinedOutput()
	require.NoError(t, err, string(out))
	defer func() {
		out, err := exec.Command("pulumi", "stack", "rm", "--yes", "-s", "testing").CombinedOutput()
		assert.NoError(t, err, string(out))
	}()

	out, err = exec.Command("pulumi", "stack", "select", "testing").CombinedOutput()
	require.NoError(t, err, string(out))

	out, err = exec.Command("pulumi", "stack", "ls").CombinedOutput()
	assert.NoError(t, err, string(out))
	assert.Contains(t, string(out), "testing*")
}

//nolint:paralleltest // this test sets the global login state
func TestAzureLoginSasToken(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})
	cloudURL := "azblob://pulumitesting?storage_account=pulumitesting"
	assert.NotEmpty(t, os.Getenv("AZURE_STORAGE_SAS_TOKEN"),
		"an azure storage SAS token needs to be set in the environment")

	t.Cleanup(func() {
		err := exec.Command("pulumi", "logout").Run()
		assert.NoError(t, err)
	})
	loginAndCreateStack(t, cloudURL)
}

//nolint:paralleltest // this test uses the global azure login state
func TestAzureLoginAzLogin(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})
	cloudURL := "azblob://pulumitesting?storage_account=pulumitesting"
	assert.NotEmpty(t, os.Getenv("AZURE_CLIENT_ID"),
		"an azure client id needs to be set in the environment")
	assert.NotEmpty(t, os.Getenv("AZURE_CLIENT_SECRET"),
		"an azure client secret needs to be set in the environment")
	assert.NotEmpty(t, os.Getenv("AZURE_TENANT_ID"),
		"an azure tenant id needs to be set in the environment")

	t.Setenv("AZURE_STORAGE_SAS_TOKEN", "")

	//nolint:gosec // this is a test
	err = exec.Command("az", "login", "--service-principal",
		"--username", os.Getenv("AZURE_CLIENT_ID"),
		"--password", os.Getenv("AZURE_CLIENT_SECRET"),
		"--tenant", os.Getenv("AZURE_TENANT_ID")).Run()
	assert.NoError(t, err)

	t.Cleanup(func() {
		err := exec.Command("az", "logout").Run()
		assert.NoError(t, err)
		err = exec.Command("pulumi", "logout").Run()
		assert.NoError(t, err)
	})

	loginAndCreateStack(t, cloudURL)
}
