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

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/stretchr/testify/assert"
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
	ctx := context.Background()
	keyName := createGCPKey(ctx, t)
	fmt.Println(keyName)
	url := "gcpkms://" + keyName
	testURL(ctx, t, url)
}
