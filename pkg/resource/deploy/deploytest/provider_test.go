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
			assert.Error(t, prov.SignalCancellation())
			assert.True(t, called)
		})
		t.Run("no CancelF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			assert.NoError(t, prov.SignalCancellation())
		})
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		prov := &Provider{}
		assert.NoError(t, prov.Close())
		// Ensure idempotent.
		assert.NoError(t, prov.Close())
	})
	t.Run("GetPluginInfo", func(t *testing.T) {
		t.Parallel()
		prov := &Provider{
			Name:    "expected-name",
			Version: semver.MustParse("1.0.0"),
		}
		info, err := prov.GetPluginInfo()
		assert.NoError(t, err)
		assert.Equal(t, "expected-name", info.Name)
		// Ensure reference is passed correctly.
		assert.Equal(t, &prov.Version, info.Version)
	})
	t.Run("GetSchema", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetSchemaF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			var called bool
			prov := &Provider{
				GetSchemaF: func(version int) ([]byte, error) {
					assert.Equal(t, 1, version)
					called = true
					return nil, expectedErr
				},
			}
			_, err := prov.GetSchema(1)
			assert.ErrorIs(t, err, expectedErr)
			assert.True(t, called)
		})
		t.Run("no GetSchemaF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			b, err := prov.GetSchema(0)
			assert.NoError(t, err)
			assert.Equal(t, []byte("{}"), b)
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
					urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					assert.Equal(t, resource.URN("expected-urn"), urn)
					assert.Equal(t, resource.NewStringProperty("old-value"), olds["old"])
					assert.Equal(t, resource.NewStringProperty("new-value"), news["new"])
					called = true
					return nil, nil, expectedErr
				},
			}
			_, _, err := prov.CheckConfig(
				resource.URN("expected-urn"),
				resource.PropertyMap{
					"old": resource.NewStringProperty("old-value"),
				},
				resource.PropertyMap{
					"new": resource.NewStringProperty("new-value"),
				},
				true,
			)
			assert.ErrorIs(t, err, expectedErr)
			assert.True(t, called)
		})
		t.Run("no CheckConfigF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			news, failures, err := prov.CheckConfig(resource.URN(""), nil /* olds */, resource.PropertyMap{
				"expected": resource.NewStringProperty("expected-value"),
			}, true)
			assert.NoError(t, err)
			assert.Empty(t, failures)
			// Should return the news.
			assert.Equal(t, resource.NewStringProperty("expected-value"), news["expected"])
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
						monitor *ResourceMonitor,
						typ, name string, parent resource.URN, inputs resource.PropertyMap,
						info plugin.ConstructInfo, options plugin.ConstructOptions,
					) (plugin.ConstructResult, error) {
						assert.Equal(t, expectedResmon, monitor)
						constructCalled = true
						return plugin.ConstructResult{}, expectedErr
					},
				}
				_, err := prov.Construct(
					plugin.ConstructInfo{
						MonitorAddress: "expected-endpoint",
					},
					tokens.Type("some-type"),
					"name",
					resource.URN("<parent-urn>"),
					nil, /* inputs */
					plugin.ConstructOptions{})
				assert.ErrorIs(t, err, expectedErr)
				assert.True(t, dialCalled)
				assert.True(t, constructCalled)
			})
			t.Run("dial error", func(t *testing.T) {
				t.Parallel()
				t.Run("invalid address", func(t *testing.T) {
					t.Parallel()
					prov := &Provider{
						ConstructF: func(
							monitor *ResourceMonitor,
							typ, name string, parent resource.URN, inputs resource.PropertyMap,
							info plugin.ConstructInfo, options plugin.ConstructOptions,
						) (plugin.ConstructResult, error) {
							assert.Fail(t, "Construct should not be called")
							return plugin.ConstructResult{}, nil
						},
					}
					_, err := prov.Construct(
						plugin.ConstructInfo{},
						tokens.Type("some-type"),
						"name",
						resource.URN("<parent-urn>"),
						nil, /* inputs */
						plugin.ConstructOptions{})
					assert.ErrorContains(t, err, "could not connect to resource monitor")
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
							monitor *ResourceMonitor,
							typ, name string, parent resource.URN, inputs resource.PropertyMap,
							info plugin.ConstructInfo, options plugin.ConstructOptions,
						) (plugin.ConstructResult, error) {
							assert.Fail(t, "Construct should not be called")
							return plugin.ConstructResult{}, nil
						},
					}
					_, err := prov.Construct(
						plugin.ConstructInfo{},
						tokens.Type("some-type"),
						"name",
						resource.URN("<parent-urn>"),
						nil, /* inputs */
						plugin.ConstructOptions{})
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
			_, err := prov.Construct(
				plugin.ConstructInfo{},
				tokens.Type("some-type"),
				"name",
				resource.URN("<parent-urn>"),
				nil, /* inputs */
				plugin.ConstructOptions{})
			assert.NoError(t, err)
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
				InvokeF: func(tok tokens.ModuleMember, inputs resource.PropertyMap,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					assert.Equal(t, tokens.ModuleMember("expected-tok"), tok)
					called = true
					return expectedPropertyMap, nil, nil
				},
			}
			res, _, err := prov.Invoke("expected-tok", nil)
			assert.NoError(t, err)
			assert.True(t, called)
			assert.Equal(t, expectedPropertyMap, res)
		})
		t.Run("no InvokeF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			news, failures, err := prov.Invoke("", nil)
			assert.NoError(t, err)
			assert.Empty(t, failures)
			assert.Equal(t, resource.PropertyMap{}, news)
		})
	})
	t.Run("StreamInvoke", func(t *testing.T) {
		t.Parallel()
		t.Run("has StreamInvokeF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			prov := &Provider{
				StreamInvokeF: func(
					tok tokens.ModuleMember, args resource.PropertyMap,
					onNext func(resource.PropertyMap) error,
				) ([]plugin.CheckFailure, error) {
					assert.Equal(t, tokens.ModuleMember("expected-tok"), tok)
					return nil, expectedErr
				},
			}
			_, err := prov.StreamInvoke("expected-tok", nil, nil)
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("no StreamInvokeF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			_, err := prov.StreamInvoke("", nil, nil)
			assert.ErrorContains(t, err, "StreamInvoke unimplemented")
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
						monitor *ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap,
						info plugin.CallInfo, options plugin.CallOptions,
					) (plugin.CallResult, error) {
						assert.Equal(t, expectedResmon, monitor)
						assert.Equal(t, tokens.ModuleMember("expected-tok"), tok)
						callCalled = true
						return plugin.CallResult{}, expectedErr
					},
				}
				_, err := prov.Call("expected-tok", nil, plugin.CallInfo{}, plugin.CallOptions{})
				assert.ErrorIs(t, err, expectedErr)
				assert.True(t, dialCalled)
				assert.True(t, callCalled)
			})
			t.Run("dial error", func(t *testing.T) {
				t.Parallel()
				t.Run("invalid address", func(t *testing.T) {
					t.Parallel()
					prov := &Provider{
						CallF: func(
							monitor *ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap,
							info plugin.CallInfo, options plugin.CallOptions,
						) (plugin.CallResult, error) {
							assert.Fail(t, "Call should not be called")
							return plugin.CallResult{}, nil
						},
					}
					_, err := prov.Call("", nil, plugin.CallInfo{}, plugin.CallOptions{})
					assert.ErrorContains(t, err, "could not connect to resource monitor")
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
						CallF: func(
							monitor *ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap,
							info plugin.CallInfo, options plugin.CallOptions,
						) (plugin.CallResult, error) {
							assert.Fail(t, "Call should not be called")
							return plugin.CallResult{}, expectedErr
						},
					}
					_, err := prov.Call("", nil, plugin.CallInfo{}, plugin.CallOptions{})
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
			_, err := prov.Call("", nil, plugin.CallInfo{}, plugin.CallOptions{})
			assert.NoError(t, err)
		})
	})
	t.Run("GetMapping", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetMappingF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			prov := &Provider{
				GetMappingF: func(key, provider string) ([]byte, string, error) {
					assert.Equal(t, "expected-key", key)
					assert.Equal(t, "expected-provider", provider)
					return nil, "", expectedErr
				},
			}
			_, _, err := prov.GetMapping("expected-key", "expected-provider")
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("no GetMappingF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			mappingB, mappingStr, err := prov.GetMapping("", "")
			assert.NoError(t, err)
			assert.Equal(t, "", mappingStr)
			assert.Nil(t, mappingB)
		})
	})
	t.Run("GetMappings", func(t *testing.T) {
		t.Parallel()
		t.Run("has GetMappingsF", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			prov := &Provider{
				GetMappingsF: func(key string) ([]string, error) {
					assert.Equal(t, "expected-key", key)
					return nil, expectedErr
				},
			}
			_, err := prov.GetMappings("expected-key")
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("no GetMappingsF", func(t *testing.T) {
			t.Parallel()
			prov := &Provider{}
			mappingStrs, err := prov.GetMappings("")
			assert.NoError(t, err)
			assert.Empty(t, mappingStrs)
		})
	})
}
