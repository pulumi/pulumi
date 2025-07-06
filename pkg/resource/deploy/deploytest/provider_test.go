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

package deploytest

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider(t *testing.T) {
	t.Parallel()
	t.Run("SignalCancellation", func(t *testing.T) {
		t.Parallel()
		t.Run("has CancelF", func(t *testing.T) {
			t.Parallel()
			var called bool
			prov := &Provider{
				CancelF: func() error {
					called = true
					return errors.New("expected error")
				},
			}
			assert.Error(t, prov.SignalCancellation(context.Background()))
			assert.True(t, called)
		})
		t.Run("no CancelF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			require.NoError(t, prov.SignalCancellation(context.Background()))
		})
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		prov := &Provider{}
		require.NoError(t, prov.Close())
		// Ensure idempotent.
		require.NoError(t, prov.Close())
	})
	t.Run("GetPluginInfo", func(t *testing.T) {
		t.Parallel()
		prov := &Provider{
			Name:    "expected-name",
			Version: semver.MustParse("1.0.0"),
		}
		info, err := prov.GetPluginInfo(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "expected-name", info.Name)
		// Ensure reference is passed correctly.
		assert.Equal(t, &prov.Version, info.Version)
	})
	t.Run("GetSchema", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetSchemaF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			expectedVersion := semver.MustParse("1.0.0")
			var called bool
			prov := &Provider{
				GetSchemaF: func(_ context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					assert.Equal(t, int32(1), req.Version)
					assert.Equal(t, "expected-subpackage", req.SubpackageName)
					assert.Equal(t, &expectedVersion, req.SubpackageVersion)
					called = true
					return plugin.GetSchemaResponse{}, expectedErr
				},
			}
			_, err := prov.GetSchema(context.Background(), plugin.GetSchemaRequest{
				Version:           1,
				SubpackageName:    "expected-subpackage",
				SubpackageVersion: &expectedVersion,
			})
			assert.ErrorIs(t, err, expectedErr)
			assert.True(t, called)
		})
		t.Run("no GetSchemaF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			b, err := prov.GetSchema(context.Background(), plugin.GetSchemaRequest{})
			require.NoError(t, err)
			assert.Equal(t, []byte("{}"), b.Schema)
		})
	})
	t.Run("CheckConfig", func(t *testing.T) {
		t.Parallel()
		t.Run("has CheckConfigF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			var called bool
			prov := &Provider{
				CheckConfigF: func(
					_ context.Context,
					req plugin.CheckConfigRequest,
				) (plugin.CheckConfigResponse, error) {
					assert.Equal(t, resource.URN("expected-urn"), req.URN)
					assert.Equal(t, resource.NewStringProperty("old-value"), req.Olds["old"])
					assert.Equal(t, resource.NewStringProperty("new-value"), req.News["new"])
					called = true
					return plugin.CheckConfigResponse{}, expectedErr
				},
			}
			_, err := prov.CheckConfig(context.Background(), plugin.CheckConfigRequest{
				URN: resource.URN("expected-urn"),
				Olds: resource.PropertyMap{
					"old": resource.NewStringProperty("old-value"),
				},
				News: resource.PropertyMap{
					"new": resource.NewStringProperty("new-value"),
				},
				AllowUnknowns: true,
			})
			assert.ErrorIs(t, err, expectedErr)
			assert.True(t, called)
		})
		t.Run("no CheckConfigF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			resp, err := prov.CheckConfig(context.Background(), plugin.CheckConfigRequest{
				News: resource.PropertyMap{
					"expected": resource.NewStringProperty("expected-value"),
				},
				AllowUnknowns: true,
			})
			require.NoError(t, err)
			assert.Empty(t, resp.Failures)
			// Should return the news.
			assert.Equal(t, resource.NewStringProperty("expected-value"), resp.Properties["expected"])
		})
	})
	t.Run("Construct", func(t *testing.T) {
		t.Parallel()
		t.Run("has ConstructF", func(t *testing.T) {
			t.Run("inject error", func(t *testing.T) {
				t.Parallel()
				expectedErr := errors.New("expected error")
				var dialCalled bool
				var constructCalled bool
				expectedResmon := &ResourceMonitor{}
				prov := &Provider{
					DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
						assert.Equal(t, "expected-endpoint", endpoint)
						dialCalled = true
						// Returns no error to avoid short-circuiting due to the monitor being provided
						// an invalid resource monitor address.
						return expectedResmon, nil
					},
					ConstructF: func(
						_ context.Context,
						req plugin.ConstructRequest,
						monitor *ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						assert.Equal(t, expectedResmon, monitor)
						constructCalled = true
						return plugin.ConstructResponse{}, expectedErr
					},
				}
				_, err := prov.Construct(context.Background(), plugin.ConstructRequest{
					Type: tokens.Type("some-type"),
					Name: "name",
					Info: plugin.ConstructInfo{
						MonitorAddress: "expected-endpoint",
					},
					Parent: resource.URN("<parent-urn>"),
				})
				assert.ErrorIs(t, err, expectedErr)
				assert.True(t, dialCalled)
				assert.True(t, constructCalled)
			})
			t.Run("connection error", func(t *testing.T) {
				t.Parallel()
				t.Run("invalid address", func(t *testing.T) {
					t.Parallel()
					prov := &Provider{
						ConstructF: func(
							_ context.Context,
							req plugin.ConstructRequest,
							_ *ResourceMonitor,
						) (plugin.ConstructResponse, error) {
							assert.Fail(t, "Construct should not be called")
							return plugin.ConstructResponse{}, nil
						},
					}
					_, err := prov.Construct(context.Background(), plugin.ConstructRequest{
						Type:   tokens.Type("some-type"),
						Name:   "name",
						Parent: resource.URN("<parent-urn>"),
					})
					assert.ErrorContains(t, err, "could not determine whether secrets are supported")
				})
				t.Run("injected error", func(t *testing.T) {
					t.Parallel()
					expectedErr := errors.New("expected error")
					var dialCalled bool
					prov := &Provider{
						DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
							dialCalled = true
							// Returns no error to avoid short-circuiting due to the monitor being provided
							// an invalid resource monitor address.
							return nil, expectedErr
						},
						ConstructF: func(
							_ context.Context,
							req plugin.ConstructRequest,
							_ *ResourceMonitor,
						) (plugin.ConstructResponse, error) {
							assert.Fail(t, "Construct should not be called")
							return plugin.ConstructResponse{}, nil
						},
					}
					_, err := prov.Construct(context.Background(), plugin.ConstructRequest{
						Type:   tokens.Type("some-type"),
						Name:   "name",
						Parent: resource.URN("<parent-urn>"),
					})
					assert.ErrorIs(t, err, expectedErr)
					assert.True(t, dialCalled)
				})
			})
		})
		t.Run("no ConstructF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{
				DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
					assert.Fail(t, "DialMonitor should not be called")
					return nil, nil
				},
			}
			_, err := prov.Construct(context.Background(), plugin.ConstructRequest{
				Type:   tokens.Type("some-type"),
				Name:   "name",
				Parent: resource.URN("<parent-urn>"),
			})
			require.NoError(t, err)
		})
	})
	t.Run("Invoke", func(t *testing.T) {
		t.Parallel()
		t.Run("has InvokeF", func(t *testing.T) {
			t.Parallel()
			expectedPropertyMap := resource.PropertyMap{
				"key": resource.NewStringProperty("expected-value"),
			}
			var called bool
			prov := &Provider{
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, tokens.ModuleMember("expected-tok"), req.Tok)
					called = true
					return plugin.InvokeResponse{Properties: expectedPropertyMap}, nil
				},
			}
			resp, err := prov.Invoke(context.Background(), plugin.InvokeRequest{
				Tok: "expected-tok",
			})
			require.NoError(t, err)
			assert.True(t, called)
			assert.Equal(t, expectedPropertyMap, resp.Properties)
		})
		t.Run("no InvokeF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			resp, err := prov.Invoke(context.Background(), plugin.InvokeRequest{})
			require.NoError(t, err)
			assert.Empty(t, resp.Failures)
			assert.Equal(t, resource.PropertyMap{}, resp.Properties)
		})
	})
	t.Run("Call", func(t *testing.T) {
		t.Parallel()
		t.Run("has CallF", func(t *testing.T) {
			t.Run("inject error", func(t *testing.T) {
				t.Parallel()
				expectedErr := errors.New("expected error")
				var dialCalled bool
				var callCalled bool
				expectedResmon := &ResourceMonitor{}
				prov := &Provider{
					DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
						dialCalled = true
						// Returns no error to avoid short-circuiting due to the monitor being provided
						// an invalid resource monitor address.
						return expectedResmon, nil
					},
					CallF: func(
						_ context.Context,
						req plugin.CallRequest,
						monitor *ResourceMonitor,
					) (plugin.CallResponse, error) {
						assert.Equal(t, expectedResmon, monitor)
						assert.Equal(t, tokens.ModuleMember("expected-tok"), req.Tok)
						callCalled = true
						return plugin.CallResponse{}, expectedErr
					},
				}
				_, err := prov.Call(context.Background(), plugin.CallRequest{Tok: "expected-tok"})
				assert.ErrorIs(t, err, expectedErr)
				assert.True(t, dialCalled)
				assert.True(t, callCalled)
			})
			t.Run("connection error", func(t *testing.T) {
				t.Parallel()
				t.Run("invalid address", func(t *testing.T) {
					t.Parallel()
					prov := &Provider{
						CallF: func(context.Context, plugin.CallRequest, *ResourceMonitor) (plugin.CallResponse, error) {
							assert.Fail(t, "Call should not be called")
							return plugin.CallResponse{}, nil
						},
					}
					_, err := prov.Call(context.Background(), plugin.CallRequest{})
					assert.ErrorContains(t, err, "could not determine whether secrets are supported")
				})
				t.Run("injected error", func(t *testing.T) {
					t.Parallel()
					expectedErr := errors.New("expected error")
					var dialCalled bool
					prov := &Provider{
						DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
							dialCalled = true
							// Returns no error to avoid short-circuiting due to the monitor being provided
							// an invalid resource monitor address.
							return nil, expectedErr
						},
						CallF: func(context.Context, plugin.CallRequest, *ResourceMonitor) (plugin.CallResponse, error) {
							assert.Fail(t, "Call should not be called")
							return plugin.CallResponse{}, expectedErr
						},
					}
					_, err := prov.Call(context.Background(), plugin.CallRequest{})
					assert.ErrorIs(t, err, expectedErr)
					assert.True(t, dialCalled)
				})
			})
		})
		t.Run("no CallF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{
				DialMonitorF: func(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
					assert.Fail(t, "Dial should not be called")
					return nil, nil
				},
			}
			_, err := prov.Call(context.Background(), plugin.CallRequest{})
			require.NoError(t, err)
		})
	})
	t.Run("GetMapping", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetMappingF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			prov := &Provider{
				GetMappingF: func(_ context.Context, req plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
					assert.Equal(t, "expected-key", req.Key)
					assert.Equal(t, "expected-provider", req.Provider)
					return plugin.GetMappingResponse{}, expectedErr
				},
			}
			_, err := prov.GetMapping(context.Background(), plugin.GetMappingRequest{
				Key:      "expected-key",
				Provider: "expected-provider",
			})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("no GetMappingF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			resp, err := prov.GetMapping(context.Background(), plugin.GetMappingRequest{})
			require.NoError(t, err)
			assert.Equal(t, "", resp.Provider)
			assert.Nil(t, resp.Data)
		})
	})
	t.Run("GetMappings", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetMappingsF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			prov := &Provider{
				GetMappingsF: func(_ context.Context, req plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error) {
					assert.Equal(t, "expected-key", req.Key)
					return plugin.GetMappingsResponse{}, expectedErr
				},
			}
			_, err := prov.GetMappings(context.Background(), plugin.GetMappingsRequest{Key: "expected-key"})
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("no GetMappingsF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			mappingStrs, err := prov.GetMappings(context.Background(), plugin.GetMappingsRequest{})
			require.NoError(t, err)
			assert.Empty(t, mappingStrs)
		})
	})
}
