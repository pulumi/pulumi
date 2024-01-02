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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getAzureCaller(ctx context.Context, t *testing.T) *azidentity.DefaultAzureCredential {
	credentials, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		t.Logf("Skipping, could not load azure config: %s", err)
		t.SkipNow()
	}
	return credentials
}

func createAzureKey(ctx context.Context, t *testing.T, credentials *azidentity.DefaultAzureCredential) string {
	url := "pulumi-testing.vault.azure.net"
	keysClient, err := azkeys.NewClient("https://"+url, credentials, nil)
	require.NoError(t, err)
	keyName := "test-key-" + randomName(t)
	keySize := int32(2048)
	kty := azkeys.JSONWebKeyTypeRSA
	params := azkeys.CreateKeyParameters{
		KeySize: &keySize,
		Kty:     &kty,
	}
	key, err := keysClient.CreateKey(ctx, keyName, params, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := keysClient.DeleteKey(ctx, keyName, nil)
		assert.NoError(t, err)
	})
	return url + "/keys/" + key.Key.KID.Name()
}

//nolint:paralleltest // mutates environment variables
func TestAzureCloudManager(t *testing.T) {
	ctx := context.Background()
	cfg := getAzureCaller(ctx, t)
	keyName := createAzureKey(ctx, t, cfg)
	url := "azurekeyvault://" + keyName

	testURL(ctx, t, url)
}

// This is a regression test for
// https://github.com/pulumi/pulumi/issues/11982.  The issue only
// appears when we have an existing key from gocloud.dev v0.27.0, and
// we try to use it with a newer version.
//
//nolint:paralleltest // mutates environment variables
func TestAzureKeyVaultExistingKey(t *testing.T) {
	ctx := context.Background()
	keyName := "pulumi-testing.vault.azure.net/keys/test-key"
	url := "azurekeyvault://" + keyName

	//nolint:lll // this is a base64 encoded key
	encryptedKeyBase64 := "Ti1qQklqTnlPTWh4RFUtNmd2WmhxcTBHeUFDa0hlS1lmNERwb3dpRHhIRlFMekxyVEdvRTZ6aFV3Q2N1Q1NISmFOeXFqajd6QzY5VmNxQzF1Z0hxRExUQUtJQUhpbE00T0ZFeXU2aUdfeS1YVE9adjlPS0M5aHlYSXdJUGwyZk01Z2FRWmJhckZfQ1kyd3lWRHlXS3JQUDcwWGFQcFBZSWJnQWJuTm5KVF9ua3gyR3I0QnBTZDVabnVrd0ViM0w1NEpjOGFqc29paVZPNVZ6OURmQ0x3MXUzVDZxTHBGLXZpV1VMTlJoQnZTMjRHdzhRWGtmczRfTzZ1NTZWdmxJRWh5TUREOF9tb2YzYlpQY0V5NW1nZDVzVjJWWHhVQWdQQlYwVDFGT2p4cGxvN1VvTUdEWUd1Q1FMcmJBS0JxbEdNZmFtSFRZcDZlYXVTQ3pUd3ptYW93"
	stackConfig := &workspace.ProjectStack{}
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
