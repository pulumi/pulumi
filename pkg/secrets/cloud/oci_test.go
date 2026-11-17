//go:build ociintegration

// Copyright 2026, Pulumi Corporation.
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
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOCICloudSecretsManager(t *testing.T) {
	t.Parallel()

	uri := os.Getenv("OCIKMS_TEST_URL")
	require.NotEmpty(t, uri, "OCIKMS_TEST_URL must select an authorized OCI key")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	info := &workspace.ProjectStack{}
	manager, err := NewCloudSecretsManager(info, uri, false)
	require.NoError(t, err)
	assert.Equal(t, uri, info.SecretsProvider)
	require.NotEmpty(t, info.EncryptedKey)

	ciphertext, err := manager.Encrypter().EncryptValue(ctx, "OCI envelope encryption regression")
	require.NoError(t, err)
	restored, err := NewCloudSecretsManagerFromState(manager.State())
	require.NoError(t, err)
	plaintext, err := restored.Decrypter().DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "OCI envelope encryption regression", plaintext)
}
