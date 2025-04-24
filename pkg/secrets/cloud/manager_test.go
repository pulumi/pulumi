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
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gosecrets "gocloud.dev/secrets"
	"gocloud.dev/secrets/driver"
)

// the main testing function, takes a kms url and tries to make a new secret manager out of it and encrypt and
// decrypt data, this is used by the aws_test and azure_test files.
func testURL(ctx context.Context, t *testing.T, url string) {
	info := &workspace.ProjectStack{}
	info.SecretsProvider = url

	var err error
	var manager secrets.Manager

	// Creating a new cloud secrets manager is sometimes flaky, so we try a few times with backoff
	// before giving up.
	for i := 1; i < 10; i++ {
		manager, err = NewCloudSecretsManager(info, url, false)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i) * 100 * time.Millisecond)
	}
	require.NoError(t, err)

	enc := manager.Encrypter()
	dec := manager.Decrypter()

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	require.NoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "plaintext", plaintext)
}

func randomName(t *testing.T) string {
	name := ""
	letters := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < 32; i++ {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		require.NoError(t, err)

		char := letters[j.Int64()]
		name = name + string(char)
	}
	return name
}

//nolint:paralleltest
func TestSecretsProviderOverride(t *testing.T) {
	// Don't call t.Parallel because we temporarily modify
	// PULUMI_CLOUD_SECRET_OVERRIDE env var and it may interfere with other
	// tests.

	stackConfig := &workspace.ProjectStack{}

	opener := &mockSecretsKeeperOpener{}
	gosecrets.DefaultURLMux().RegisterKeeper("test", opener)

	//nolint:paralleltest
	t.Run("without override", func(t *testing.T) {
		opener.wantURL = "test://foo"
		_, createSecretsManagerError := NewCloudSecretsManager(stackConfig, "test://foo", false)
		assert.Nil(t, createSecretsManagerError, "Creating the cloud secret manager should succeed")

		_, createSecretsManagerError = NewCloudSecretsManager(stackConfig, "test://bar", false)
		msg := "NewCloudSecretsManager with unexpected secretsProvider URL succeeded, expected an error"
		assert.NotNil(t, createSecretsManagerError, msg)
	})

	//nolint:paralleltest
	t.Run("with override", func(t *testing.T) {
		opener.wantURL = "test://bar"
		t.Setenv("PULUMI_CLOUD_SECRET_OVERRIDE", "test://bar")

		// Last argument here shouldn't matter anymore, since it gets overridden
		// by the env var. Both calls should succeed.
		msg := "creating the secrets manager should succeed regardless of secrets provider"
		_, createSecretsManagerError := NewCloudSecretsManager(stackConfig, "test://foo", false)
		assert.Nil(t, createSecretsManagerError, msg)
		_, createSecretsManagerError = NewCloudSecretsManager(stackConfig, "test://bar", false)
		assert.Nil(t, createSecretsManagerError, msg)
	})
}

type mockSecretsKeeperOpener struct {
	wantURL string
}

func (m *mockSecretsKeeperOpener) OpenKeeperURL(ctx context.Context, u *url.URL) (*gosecrets.Keeper, error) {
	if m.wantURL != u.String() {
		return nil, fmt.Errorf("got keeper URL: %q, want: %q", u, m.wantURL)
	}
	return gosecrets.NewKeeper(dummySecretsKeeper{}), nil
}

type dummySecretsKeeper struct {
	driver.Keeper
}

func (k dummySecretsKeeper) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (k dummySecretsKeeper) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}
