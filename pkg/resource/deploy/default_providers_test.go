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

package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestDefaultProviders(t *testing.T) {
	t.Parallel()
	t.Run("normalizeProviderRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("use defaultProvider", func(t *testing.T) {
			t.Parallel()
			v1 := semver.MustParse("0.1.0")
			d := &defaultProviders{
				defaultProviderInfo: map[tokens.Package]workspace.PackageDescriptor{
					tokens.Package("pkg"): {
						PluginDescriptor: workspace.PluginDescriptor{
							Version:           &v1,
							PluginDownloadURL: "github://owner/repo",
							Checksums:         map[string][]byte{"key": []byte("expected-checksum-value")},
						},
					},
				},
				config: &configSourceMock{},
			}
			req := d.normalizeProviderRequest(providers.NewProviderRequest(tokens.Package("pkg"), nil, "", nil, nil))
			require.NotNil(t, req)
			assert.Equal(t, &v1, req.Version())
			assert.Equal(t, "github://owner/repo", req.PluginDownloadURL())
			assert.Equal(t, map[string][]byte{"key": []byte("expected-checksum-value")}, req.PluginChecksums())
		})
	})
	t.Run("newRegisterDefaultProviderEvent", func(t *testing.T) {
		t.Parallel()
		t.Run("error in GetPackageConfig()", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
						return property.Map{}, expectedErr
					},
				},
			}
			_, _, err := d.newRegisterDefaultProviderEvent(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
	})
	t.Run("handleRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("error in shouldDenyRequest", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
						return property.Map{}, expectedErr
					},
				},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("error in newRegisterDefaultProviderEvent", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
						if pkg == "pulumi" {
							// Enables shouldDenyRequest(req) to succeed as it always calls using
							// "pulumi".
							return property.Map{}, nil
						}
						return property.Map{}, expectedErr
					},
				},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("error due to cancel before registration", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
				config: &configSourceMock{},
			}
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
		t.Run("error cancel after registration, but before registration result", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)

			providerRegChan := make(chan *registerResourceEvent, 1)
			d := &defaultProviders{
				cancel:          cancel,
				providerRegChan: providerRegChan,
				config:          &configSourceMock{},
			}
			go func() {
				// Cancel after reading the registration.
				<-providerRegChan
				cancel <- true
			}()
			_, err := d.handleRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
	})
	t.Run("shouldDenyRequest", func(t *testing.T) {
		t.Parallel()
		t.Run("GetPackageConfigErr", func(t *testing.T) {
			t.Parallel()

			expectedErr := errors.New("expected error")
			d := &defaultProviders{
				config: &configSourceMock{
					GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
						return property.Map{}, expectedErr
					},
				},
			}
			_, err := d.shouldDenyRequest(providers.ProviderRequest{})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("disable-default-providers", func(t *testing.T) {
			t.Parallel()
			t.Run("invalid value", func(t *testing.T) {
				t.Parallel()
				d := &defaultProviders{
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
							return property.NewMap(map[string]property.Value{
								"disable-default-providers": property.New(100.0),
							}), nil
						},
					},
				}
				_, err := d.shouldDenyRequest(providers.ProviderRequest{})
				assert.ErrorContains(t, err, "Unexpected encoding of pulumi:disable-default-providers")
			})
			t.Run("empty value", func(t *testing.T) {
				t.Parallel()
				d := &defaultProviders{
					config: &configSourceMock{
						GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
							return property.NewMap(map[string]property.Value{
								"disable-default-providers": property.New(""),
							}), nil
						},
					},
				}
				res, err := d.shouldDenyRequest(providers.ProviderRequest{})
				require.NoError(t, err)
				assert.False(t, res)
			})
			t.Run("invalid list", func(t *testing.T) {
				t.Run("bad json", func(t *testing.T) {
					t.Parallel()
					d := &defaultProviders{
						config: &configSourceMock{
							GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
								return property.NewMap(map[string]property.Value{
									"disable-default-providers": property.New("[[["),
								}), nil
							},
						},
					}
					res, err := d.shouldDenyRequest(providers.ProviderRequest{})
					assert.ErrorContains(t, err, "Failed to parse [[[")
					assert.True(t, res)
				})
				t.Run("mixed list values", func(t *testing.T) {
					t.Parallel()
					d := &defaultProviders{
						config: &configSourceMock{
							GetPackageConfigF: func(pkg tokens.Package) (property.Map, error) {
								return property.NewMap(map[string]property.Value{
									"disable-default-providers": property.New(`["foo", 2, 3]`),
								}), nil
							},
						},
					}
					res, err := d.shouldDenyRequest(providers.ProviderRequest{})
					assert.ErrorContains(t, err, "must be a string")
					assert.True(t, res)
				})
			})
		})
	})
	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		t.Run("serve respects cancel", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
			}
			d.serve()
		})
		t.Run("getDefaultProviderRef respects cancel", func(t *testing.T) {
			t.Parallel()
			cancel := make(chan bool, 1)
			cancel <- true
			d := &defaultProviders{
				cancel: cancel,
			}
			_, err := d.getDefaultProviderRef(providers.ProviderRequest{})
			assert.ErrorIs(t, err, context.Canceled)
		})
	})
}

func TestParseProviderRequest(t *testing.T) {
	t.Parallel()
	t.Run("bad version", func(t *testing.T) {
		t.Parallel()
		_, err := parseProviderRequest("", "bad-version", "", nil, nil)
		assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
	})
}
