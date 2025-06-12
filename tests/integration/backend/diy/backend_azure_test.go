// Copyright 2024-2024, Pulumi Corporation.
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

	token := os.Getenv("AZURE_STORAGE_SAS_TOKEN")
	if token == "" {
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
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")
	if clientID == "" || clientSecret == "" || tenantID == "" {
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
