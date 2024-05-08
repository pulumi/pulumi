package diy

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // this test sets the global login state
func TestAzureLoginSasToken(t *testing.T) {
	err := os.Chdir("project")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Chdir("..")
		require.NoError(t, err)
	})
	cloudURL := "azblob://pulumitesting?storage_account=pulumitesting"

	// Make sure we use the SAS token for login here
	t.Setenv("AZURE_CLIENT_ID", "")
	t.Setenv("AZURE_CLIENT_SECRET", "")
	t.Setenv("AZURE_TENANT_ID", "")

	_, ok := os.LookupEnv("AZURE_STORAGE_SAS_TOKEN")
	if !ok {
		t.Skip("AZURE_STORAGE_SAS_TOKEN not set, skipping test")
	}

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
	_, clientIDSet := os.LookupEnv("AZURE_CLIENT_ID")
	_, clientSecretSet := os.LookupEnv("AZURE_CLIENT_SECRET")
	_, tenantIDSet := os.LookupEnv("AZURE_TENANT_ID")
	if !clientIDSet || !clientSecretSet || !tenantIDSet {
		t.Skip("AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, and AZURE_TENANT_ID not set, skipping test")
	}

	// Make sure we don't use the SAS token for login here
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
