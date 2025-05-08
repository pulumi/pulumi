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

package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
)

func TestTarget(t *testing.T) {
	t.Parallel()
	t.Run("GetPackageConfig", func(t *testing.T) {
		t.Parallel()
		t.Run("bad crypter", func(t *testing.T) {
			t.Parallel()
			t.Run("secret in namespace", func(t *testing.T) {
				t.Parallel()
				expectedErr := errors.New("expected error")
				target := &Target{
					Config: config.Map{
						config.MustMakeKey("test", "secret"):  config.NewSecureValue("secret-value"),
						config.MustMakeKey("test", "regular"): config.NewValue("secret-value"),
					},
					Decrypter: &decrypterMock{
						DecryptValueF: func(
							ctx context.Context, ciphertext string,
						) (string, error) {
							return "", expectedErr
						},
					},
				}
				_, err := target.GetPackageConfig("test")
				assert.ErrorIs(t, err, expectedErr)
			})
			//nolint:paralleltest // golangci-lint v2 upgrade
			t.Run("different namespace", func(t *testing.T) {
				target := &Target{
					Config: config.Map{
						config.MustMakeKey("a", "val"): config.NewSecureValue("secret-value"),
						config.MustMakeKey("b", "val"): config.NewValue("plain-value"),
					},
					Decrypter: &decrypterMock{},
				}
				_, err := target.GetPackageConfig("something-else")
				assert.NoError(t, err)
			})
		})

		t.Run("ok", func(t *testing.T) { //nolint:paralleltest // golangci-lint v2 upgrade
			expectedErr := errors.New("expected error")
			target := &Target{
				Config: config.Map{
					config.MustMakeKey("a", "val"):        config.NewValue("a-value"),
					config.MustMakeKey("b", "val"):        config.NewValue("b-value"),
					config.MustMakeKey("c", "val"):        config.NewValue("c-value"),
					config.MustMakeKey("test", "secret"):  config.NewSecureValue("secret-value"),
					config.MustMakeKey("test", "regular"): config.NewValue("regular-value"),
				},
				Decrypter: &decrypterMock{
					DecryptValueF: func(
						ctx context.Context, ciphertext string,
					) (string, error) {
						return ciphertext, nil
					},
				},
			}
			res, err := target.GetPackageConfig("test")
			assert.NoError(t, err, expectedErr)

			cfg := res.Mappable()
			assert.Equal(t, "regular-value", cfg["regular"])
			secret, ok := cfg["secret"].(*resource.Secret)
			assert.True(t, ok)
			assert.Equal(t, "secret-value", secret.Element.V)

			_, ok = cfg["a"].(*resource.Secret)
			assert.False(t, ok)
		})
	})
}
