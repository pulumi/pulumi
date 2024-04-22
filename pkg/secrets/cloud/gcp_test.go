// Copyright 2016-2023, Pulumi Corporation.
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

package cloud

import (
	"context"
	"os"
	"testing"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createGCPKey(ctx context.Context, t *testing.T) string {
	keyName := "test-key-" + randomName(t)

	parent := "projects/pulumi-development/locations/global/keyRings/pulumi-testing"
	client, err := kms.NewKeyManagementClient(ctx)
	assert.NoError(t, err)

	// Build the request.
	req := &kmspb.CreateCryptoKeyRequest{
		Parent:      parent,
		CryptoKeyId: keyName,
		CryptoKey: &kmspb.CryptoKey{
			Purpose: kmspb.CryptoKey_ENCRYPT_DECRYPT,
			VersionTemplate: &kmspb.CryptoKeyVersionTemplate{
				Algorithm: kmspb.CryptoKeyVersion_GOOGLE_SYMMETRIC_ENCRYPTION,
			},
		},
	}

	// Call the API.
	result, err := client.CreateCryptoKey(ctx, req)
	assert.NoError(t, err)
	t.Cleanup(func() {
		_, err := client.DestroyCryptoKeyVersion(ctx, &kmspb.DestroyCryptoKeyVersionRequest{
			Name: result.Name + "/cryptoKeyVersions/1",
		})
		assert.NoError(t, err)
		client.Close()
	})
	return result.Name
}

//nolint:paralleltest // mutates environment variables
func TestGCPCloudManager(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	ctx := context.Background()
	keyName := createGCPKey(ctx, t)
	url := "gcpkms://" + keyName
	testURL(ctx, t, url)
}

//nolint:paralleltest // mutates environment variables
func TestGCPExistingKey(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	ctx := context.Background()

	url := "gcpkms://projects/pulumi-development/locations/global/keyRings/pulumi-testing/cryptoKeys/pulumi-ci-test-key"

	//nolint:lll // this is a base64 encoded key
	encryptedKeyBase64 := "CiQAAVPx+1LGEXNyhMLo89JUdLIUqqsHxB3GlqHHqsGgQB2O7IYSSQBzSboprGFFkoJKRp5baCnFKH5gkCiADJINnUF9luzY93RjYSlyQ23qj0kopX3ZuuXB+ZuzSEqaH0IOL9RoYP1kB+FIXGdkWXE="
	stackConfig := &workspace.ProjectStack{}
	stackConfig.SecretsProvider = url
	stackConfig.EncryptedKey = encryptedKeyBase64
	manager, err := NewCloudSecretsManager(stackConfig, url, false)
	require.NoError(t, err)

	enc, err := manager.Encrypter()
	require.NoError(t, err)

	dec, err := manager.Decrypter()
	require.NoError(t, err)

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)
}

//nolint:paralleltest // mutates environment variables
func TestGCPExistingState(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	ctx := context.Background()

	//nolint:lll // this includes a base64 encoded key
	cloudState := `{
		"url": "gcpkms://projects/pulumi-development/locations/global/keyRings/pulumi-testing/cryptoKeys/pulumi-ci-test-key",
		"encryptedkey": "CiQAAVPx+1LGEXNyhMLo89JUdLIUqqsHxB3GlqHHqsGgQB2O7IYSSQBzSboprGFFkoJKRp5baCnFKH5gkCiADJINnUF9luzY93RjYSlyQ23qj0kopX3ZuuXB+ZuzSEqaH0IOL9RoYP1kB+FIXGdkWXE="
	}`
	manager, err := NewCloudSecretsManagerFromState([]byte(cloudState))
	require.NoError(t, err)

	enc, err := manager.Encrypter()
	require.NoError(t, err)

	dec, err := manager.Decrypter()
	require.NoError(t, err)

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)

	assert.JSONEq(t, cloudState, string(manager.State()))
}

//nolint:paralleltest // mutates environment variables
func TestGCPKeyEditProjectStack(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	url := "gcpkms://projects/pulumi-development/locations/global/keyRings/pulumi-testing/cryptoKeys/pulumi-ci-test-key"

	//nolint:lll // this is a base64 encoded key
	encryptedKeyBase64 := "CiQAAVPx+1LGEXNyhMLo89JUdLIUqqsHxB3GlqHHqsGgQB2O7IYSSQBzSboprGFFkoJKRp5baCnFKH5gkCiADJINnUF9luzY93RjYSlyQ23qj0kopX3ZuuXB+ZuzSEqaH0IOL9RoYP1kB+FIXGdkWXE="

	stackConfig := &workspace.ProjectStack{}
	stackConfig.SecretsProvider = url
	stackConfig.EncryptedKey = encryptedKeyBase64
	manager, err := NewCloudSecretsManager(stackConfig, url, false)
	require.NoError(t, err)

	newConfig := &workspace.ProjectStack{}
	err = EditProjectStack(newConfig, manager.State())
	require.NoError(t, err)

	assert.Equal(t, stackConfig.EncryptedKey, newConfig.EncryptedKey)
}
