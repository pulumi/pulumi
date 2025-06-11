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
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getAzureCaller(ctx context.Context, t *testing.T) *azidentity.DefaultAzureCredential {
	credentials, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		t.Skipf("Skipping, could not load azure config: %s", err)
	}
	_, err = credentials.GetToken(ctx, policy.TokenRequestOptions{})
	if err != nil && strings.Contains(err.Error(), "failed to acquire a token") {
		t.Skipf("Skipping, could not acquire Azure token: %s", err)
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
	stackConfig.SecretsProvider = url
	stackConfig.EncryptedKey = encryptedKeyBase64
	manager, err := NewCloudSecretsManager(stackConfig, url, false)
	require.NoError(t, err)

	enc := manager.Encrypter()
	dec := manager.Decrypter()

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)
}

//nolint:paralleltest // mutates environment variables
func TestAzureKeyVaultExistingState(t *testing.T) {
	ctx := context.Background()

	//nolint:lll // this includes a base64 encoded key
	cloudState := `{
                    "url": "azurekeyvault://pulumi-testing.vault.azure.net/keys/test-key",
                    "encryptedkey": "c3YwTUQwWjRHYlZXYWpIX3k2Z2twNDFXTlBlRERVOHJKakdTbWFVdk44NWNBWHdQTUJrNFBsRjl5SXJCaEZ5T1VjSVVCTmVkV3NOaWxCbTBOcnotYjVXOHFMRHdWbk9oQTZDMGNyX1p3ZEhiRW1acVpTX1RjazFLd0RmM1RPc3c3Q2s1UTduSXV1NERQYjdUU3ZSdzlDNU5UZ01RRXRhSWgtVGdvM3U0aXVMZV8zY3d5eHhwamIyRW8tYWEyS29XeEpSNXhUb3gxaUQ0dUFfbEszdEx0bXQxVWpod1BCZUJHcmVWd2RmclBUUk1Jcm52ZkZMaS1nQXpLT1VwLVJMQURpX1pVdU9BUFRnLWR2OGpUeGUxNV9pSkN1U25BVUstY2RRemtpRVVheGlsUVlhX2lOa0JXYm95ZUpubEM3eFhNalAwVGhQMUp4ZUI0bmxpcW9YUFFB"
}
`
	manager, err := NewCloudSecretsManagerFromState([]byte(cloudState))
	require.NoError(t, err)

	enc := manager.Encrypter()
	dec := manager.Decrypter()

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)

	assert.JSONEq(t, cloudState, string(manager.State()))
}

//nolint:paralleltest // mutates environment variables
func TestAzureKeyEditProjectStack(t *testing.T) {
	keyName := "pulumi-testing.vault.azure.net/keys/test-key"
	url := "azurekeyvault://" + keyName

	//nolint:lll // this is a base64 encoded key
	encryptedKeyBase64 := "cTNpdy1GazRQcklya0gzWFZ6N2hFQXRaOFVfZm1heVZKQVlxQmQwMGh1V0dGNGc4OHhJUXQxVUVmdmViY0xtY01ubEx0bW5tOExEZ0F1VEFLdHgzRjF6S1NDa3EyZ3RHdFFycHk0aUJQWDRFS2lpV2tKMl9WS0lCUnB4QmhmQTJacHBvT1ZUVVZQRWU0Zm1sV1pod3Y4REd5M2p3Vnh3OFFHM2ptdXRRNnJXUzRjVEZGTXpFd3JWeFE5dlo1YTcwWFBIV3o5UFU4SjBGX0dIdlJFSFJpSmJ5c3Q0bS1fenJ6T002RTZacFp0LTVZdl9IT1d5LUo1SkxpbG5VYnFHU2lvbFNpeE9iQ2hWdGk3R28zTlM4ZkQxS2lQVnVMeUJTTDZMNmdoSGZoQXBGdnpwdUJQMWRsTlRaaHZpY0VBa2RQblpYNGJXWVAxTk5yTG5DaHpWeDlB"
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

// Regression test for https://github.com/pulumi/pulumi/issues/15329
//
//nolint:paralleltest // mutates environment variables
func TestAzureKeyVaultExistingKeyState(t *testing.T) {
	ctx := context.Background()
	keyName := "pulumi-testing.vault.azure.net/keys/test-key"
	url := "azurekeyvault://" + keyName

	//nolint:lll // this is a base64 encoded key
	encryptedKeyBase64 := "aHROMWFlam5qX0xjVTl3WVhQdzU1alJVbWVvaFU3UENfbzk3dDU2d2FRdUJvYWR1Z0pwdHhiRDU1akRuWFFUdFVPeWNMdlFlemh1UE9IN0txV21RU3NYZDJha0xscWp5RFFTNGQtQ2lhOXRJOGgtSnd0ZHAyOWdkNEx1ejBjVmRvY3NSUlZhdnhtZkNnMTd2TG9vZ0tfbG02Wi1VYnl2Z0xraGNRVzl0T0s2c3BScjdQX2E4NzRaMV8zeTQyb3lLUWx6U1RlYnNmS0xRRDBoZENsT0VSaGlTTHRxazlzMnlKTGpEZ2Q4VUVTSnFzaG9XY2JkVFBnX2NXcWpnQVVjSTRhOEllckE2Z0Y5YXh1eW9DVndoaS1GNGJiN1NPRW5MTEVhZUtIVTZjVFFHeGFoLV9FeVlwTEZKX3dxYzNsRDZ4aU1RdVh0blQ2WG9tZXQ0V3NQMmVn"
	stackConfig := &workspace.ProjectStack{}
	stackConfig.SecretsProvider = url
	stackConfig.EncryptedKey = encryptedKeyBase64
	manager, err := NewCloudSecretsManager(stackConfig, url, false)
	require.NoError(t, err)

	enc := manager.Encrypter()
	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	// Now create a new manager from the state of the old one.
	newManager, err := NewCloudSecretsManagerFromState(manager.State())
	require.NoError(t, err)

	dec := newManager.Decrypter()
	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)
}

// Test to auto-fix the regressions from https://github.com/pulumi/pulumi/issues/15329
//
//nolint:paralleltest // mutates environment variables
func TestAzureKeyVaultAutoFix15329(t *testing.T) {
	ctx := context.Background()
	// https://github.com/pulumi/pulumi/issues/15329 result in keys like the following being written to state. We can auto-detect these because they aren't valid base64.
	//nolint:lll // this includes a base64 encoded key
	encryptedKey := "nLdkXrvtOYvgaVHn8FrdALMtFjgV67KoGIb6kWwz5Weo/yxAVyK7Rl0rtNxoIDnOvkvRQdCDTSrq1q8w6XZU/cvZ5FQMTMN3l1I28r7YV4HIBzDxx0G964DrfUSlxh1GhpQogcLiYor9MCGEidd5BdAqxKMHZJXUGJLCoUuuA3kWBwkeAowstpkumfXzxgxocq2BIkrfPqkfSetmLQajhBNn9dAIgxhaIaM+ubjOAFHkvYlrujE8dY7b2wNVa2ua/3tYfyIBYyg8jFRdOjxXXpXs7cZcRD3oQxa3F1DxYPl/IxuQdyHWxvmYH9SXVKn/B1z7JcOraZDTAptDgc3B0Q=="
	cloudState := fmt.Sprintf(`{
		"url": "azurekeyvault://pulumi-testing.vault.azure.net/keys/test-key",
		"encryptedkey": "%s"
}
`, encryptedKey)

	manager, err := NewCloudSecretsManagerFromState([]byte(cloudState))
	require.NoError(t, err)

	enc := manager.Encrypter()
	dec := manager.Decrypter()

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)

	// This should now write out a _fixed_ key to state
	stackConfig := &workspace.ProjectStack{}
	err = EditProjectStack(stackConfig, manager.State())
	require.NoError(t, err)

	// This is the actual expected key we want, it's the same as the one above but with the base64 encoding fixed so it's double wrapped as we expect.
	//nolint:lll // this includes a base64 encoded key
	expectedKey := "bkxka1hydnRPWXZnYVZIbjhGcmRBTE10RmpnVjY3S29HSWI2a1d3ejVXZW9feXhBVnlLN1JsMHJ0TnhvSURuT3ZrdlJRZENEVFNycTFxOHc2WFpVX2N2WjVGUU1UTU4zbDFJMjhyN1lWNEhJQnpEeHgwRzk2NERyZlVTbHhoMUdocFFvZ2NMaVlvcjlNQ0dFaWRkNUJkQXF4S01IWkpYVUdKTENvVXV1QTNrV0J3a2VBb3dzdHBrdW1mWHp4Z3hvY3EyQklrcmZQcWtmU2V0bUxRYWpoQk5uOWRBSWd4aGFJYU0tdWJqT0FGSGt2WWxydWpFOGRZN2Iyd05WYTJ1YV8zdFlmeUlCWXlnOGpGUmRPanhYWHBYczdjWmNSRDNvUXhhM0YxRHhZUGxfSXh1UWR5SFd4dm1ZSDlTWFZLbl9CMXo3SmNPcmFaRFRBcHREZ2MzQjBR"
	assert.Equal(t, expectedKey, stackConfig.EncryptedKey)
}

// Test that the auto-fix for #15329 doesn't ruin error messages for invalid data
//
//nolint:paralleltest // mutates environment variables
func TestAzureKeyVaultKeyError(t *testing.T) {
	cloudState := `{
		"url": "azurekeyvault://pulumi-testing.vault.azure.net/keys/test-key",
		"encryptedkey": "not base64 data"
}
`

	_, err := NewCloudSecretsManagerFromState([]byte(cloudState))
	require.ErrorContains(t, err, "unmarshalling state: illegal base64 data at input byte 3")
}
