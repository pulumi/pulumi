// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"gocloud.dev/secrets"
	"gocloud.dev/secrets/driver"
)

func TestSecretsproviderOverride(t *testing.T) {
	// Don't call t.Parallel because we temporarily modify
	// PULUMI_CLOUD_SECRET_OVERRIDE env var and it may interfere with other
	// tests.

	const stackConfig = "Pulumi.TestSecretsproviderOverride.yaml"
	var stackName = tokens.Name("TestSecretsproviderOverride")
	// Cleanup the generated stack config after the test.
	t.Cleanup(func() { os.Remove(stackConfig) })

	opener := &mockSecretsKeeperOpener{}
	secrets.DefaultURLMux().RegisterKeeper("test", opener)

	t.Run("without override", func(t *testing.T) {
		opener.wantURL = "test://foo"

		if _, err := newCloudSecretsManager(stackName, stackConfig, "test://foo"); err != nil {
			t.Fatalf("newCloudSecretsManager failed: %v", err)
		}
		if _, err := newCloudSecretsManager(stackName, stackConfig, "test://bar"); err == nil {
			t.Fatal("newCloudSecretsManager with unexpected secretsProvider URL succeeded, expected an error")
		}
	})

	t.Run("with override", func(t *testing.T) {
		opener.wantURL = "test://bar"
		t.Setenv("PULUMI_CLOUD_SECRET_OVERRIDE", "test://bar")

		// Last argument here shouldn't matter anymore, since it gets overriden
		// by the env var. Both calls should succeed.
		if _, err := newCloudSecretsManager(stackName, stackConfig, "test://foo"); err != nil {
			t.Fatalf("newCloudSecretsManager failed: %v", err)
		}
		if _, err := newCloudSecretsManager(stackName, stackConfig, "test://bar"); err != nil {
			t.Fatalf("newCloudSecretsManager failed: %v", err)
		}
	})
}

type mockSecretsKeeperOpener struct {
	wantURL string
}

func (m *mockSecretsKeeperOpener) OpenKeeperURL(ctx context.Context, u *url.URL) (*secrets.Keeper, error) {
	if m.wantURL != u.String() {
		return nil, fmt.Errorf("got keeper URL: %q, want: %q", u, m.wantURL)
	}
	return secrets.NewKeeper(dummySecretsKeeper{}), nil
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
