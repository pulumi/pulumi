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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys"

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
	fmt.Printf("%s", key.Key.KID.Name())
	return url + "/keys/" + keyName
}

//nolint:paralleltest // mutates environment variables
func TestAzureCloudManager(t *testing.T) {
	ctx := context.Background()
	cfg := getAzureCaller(ctx, t)
	keyName := createAzureKey(ctx, t, cfg)
	url := "azurekeyvault://" + keyName

	testURL(ctx, t, url)
}
