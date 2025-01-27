// Copyright 2016-2024, Pulumi Corporation.
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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestBuiltinProvider(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		assert.NoError(t, p.Close())
	})
	t.Run("Pkg", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		assert.Equal(t, tokens.Package("pulumi"), p.Pkg())
	})
	t.Run("GetSchema", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		b, err := p.GetSchema(context.Background(), plugin.GetSchemaRequest{})
		assert.NoError(t, err)
		assert.Equal(t, []byte("{}"), b.Schema)
	})
	t.Run("GetMapping", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		m, err := p.GetMapping(context.Background(), plugin.GetMappingRequest{Key: "key", Provider: "provider"})
		assert.NoError(t, err)
		assert.Nil(t, m.Data)
		assert.Equal(t, "", m.Provider)
	})
	t.Run("GetMappings", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		strs, err := p.GetMappings(context.Background(), plugin.GetMappingsRequest{Key: "key"})
		assert.NoError(t, err)
		assert.Empty(t, strs)
	})
	t.Run("Check", func(t *testing.T) {
		t.Parallel()
		t.Run("builtin only supports stack reference type", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{}
			_, err := p.Check(context.Background(), plugin.CheckRequest{
				URN:           resource.CreateURN("foo", "not-stack-reference-type", "", "proj", "stack"),
				Olds:          resource.PropertyMap{},
				News:          resource.PropertyMap{},
				AllowUnknowns: true,
			})
			assert.ErrorContains(t, err, "unrecognized resource type")
		})
		t.Run("missing `name` input property", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			resp, err := p.Check(context.Background(), plugin.CheckRequest{
				URN:           resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				Olds:          resource.PropertyMap{},
				News:          resource.PropertyMap{},
				AllowUnknowns: true,
			})
			assert.Equal(t, []plugin.CheckFailure{
				{
					Property: "name",
					Reason:   `missing required property "name"`,
				},
			}, resp.Failures)
			assert.NoError(t, err)
		})
		t.Run(`property "name" must be a string`, func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			resp, err := p.Check(context.Background(), plugin.CheckRequest{
				URN:  resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				Olds: resource.PropertyMap{},
				News: resource.PropertyMap{
					"name": resource.NewNumberProperty(10),
				},
				AllowUnknowns: true,
			})
			assert.Equal(t, []plugin.CheckFailure{
				{
					Property: "name",
					Reason:   `property "name" must be a string`,
				},
			}, resp.Failures)
			assert.NoError(t, err)
		})
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			resp, err := p.Check(context.Background(), plugin.CheckRequest{
				URN: resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				News: resource.PropertyMap{
					"name": resource.NewStringProperty("res-name"),
				},
				AllowUnknowns: true,
			})
			assert.Nil(t, resp.Failures)
			assert.NoError(t, err)
			assert.Equal(t, resource.PropertyMap{
				"name": resource.NewStringProperty("res-name"),
			}, resp.Properties)
		})
	})
	t.Run("Update (always fails)", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			p := &builtinProvider{}

			oldOutputs := resource.PropertyMap{"cookie": resource.NewStringProperty("yum")}
			_, err := p.Update(context.Background(), plugin.UpdateRequest{
				URN:        resource.CreateURN("foo", "not-stack-reference-type", "", "proj", "stack"),
				ID:         "some-id",
				OldInputs:  nil,
				OldOutputs: oldOutputs,
				NewInputs:  resource.PropertyMap{},
			})
			contract.Ignore(err)
		})
	})
	t.Run("Construct (always fails)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.Construct(context.Background(), plugin.ConstructRequest{Inputs: resource.PropertyMap{}})
		assert.ErrorContains(t, err, "builtin resources may not be constructed")
	})
	t.Run("Invoke", func(t *testing.T) {
		t.Parallel()
		t.Run(readStackOutputs, func(t *testing.T) {
			t.Parallel()
			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{}
				_, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: readStackOutputs,
					Args: resource.PropertyMap{
						"name": resource.NewStringProperty("res-name"),
					},
				})
				assert.ErrorContains(t, err, "no backend client is available")
			})
			t.Run("ok", func(t *testing.T) {
				t.Parallel()
				var called bool
				p := &builtinProvider{
					backendClient: &deploytest.BackendClient{
						GetStackOutputsF: func(ctx context.Context, name string) (resource.PropertyMap, error) {
							called = true
							return resource.PropertyMap{
								"normal": resource.NewStringProperty("foo"),
								"secret": resource.MakeSecret(resource.NewStringProperty("bar")),
							}, nil
						},
					},
				}
				resp, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: readStackOutputs,
					Args: resource.PropertyMap{
						"name": resource.NewStringProperty("res-name"),
					},
				})
				assert.NoError(t, err)
				assert.True(t, called)
				assert.Nil(t, resp.Failures)

				assert.Equal(t, "res-name", resp.Properties["name"].V)

				assert.Equal(t, "foo", resp.Properties["outputs"].ObjectValue()["normal"].StringValue())
				assert.Len(t, resp.Properties["secretOutputNames"].V, 1)
			})
		})
		t.Run(readStackResourceOutputs, func(t *testing.T) {
			t.Parallel()
			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{}
				_, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: readStackResourceOutputs,
					Args: resource.PropertyMap{
						"stackName": resource.NewStringProperty("res-name"),
					},
				})
				assert.ErrorContains(t, err, "no backend client is available")
			})
			t.Run("ok", func(t *testing.T) {
				t.Parallel()
				var called bool
				p := &builtinProvider{
					backendClient: &deploytest.BackendClient{
						GetStackResourceOutputsF: func(ctx context.Context, name string) (resource.PropertyMap, error) {
							called = true
							return resource.PropertyMap{}, nil
						},
					},
				}
				_, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: readStackResourceOutputs,
					Args: resource.PropertyMap{
						"stackName": resource.NewStringProperty("res-name"),
					},
				})
				assert.NoError(t, err)
				assert.True(t, called)
			})
		})
		t.Run(getResource, func(t *testing.T) {
			t.Parallel()

			t.Run("ok new", func(t *testing.T) {
				t.Parallel()

				p := &builtinProvider{
					news:  &gsync.Map[urn.URN, *resource.State]{},
					reads: &gsync.Map[urn.URN, *resource.State]{},
				}

				expected := &resource.State{
					Outputs: resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
					},
				}

				p.news.Store("res-name", expected)

				actual, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: getResource,
					Args: resource.PropertyMap{
						"urn": resource.NewStringProperty("res-name"),
					},
				})

				assert.NoError(t, err)
				assert.Equal(t, expected.Outputs, actual.Properties["state"].ObjectValue())
			})

			t.Run("ok read", func(t *testing.T) {
				t.Parallel()

				p := &builtinProvider{
					news:  &gsync.Map[urn.URN, *resource.State]{},
					reads: &gsync.Map[urn.URN, *resource.State]{},
				}

				expected := &resource.State{
					Outputs: resource.PropertyMap{
						"foo": resource.NewStringProperty("bar"),
					},
				}

				p.reads.Store("res-name", expected)

				actual, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: getResource,
					Args: resource.PropertyMap{
						"urn": resource.NewStringProperty("res-name"),
					},
				})

				assert.NoError(t, err)
				assert.Equal(t, expected.Outputs, actual.Properties["state"].ObjectValue())
			})

			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{
					news:  &gsync.Map[urn.URN, *resource.State]{},
					reads: &gsync.Map[urn.URN, *resource.State]{},
				}
				_, err := p.Invoke(context.Background(), plugin.InvokeRequest{
					Tok: getResource,
					Args: resource.PropertyMap{
						"urn": resource.NewStringProperty("res-name"),
					},
				})
				assert.ErrorContains(t, err, "unknown resource")
			})
		})
	})
	t.Run("StreamInvoke (unimplemented)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.StreamInvoke(context.Background(), plugin.StreamInvokeRequest{})
		assert.ErrorContains(t, err, "the builtin provider does not implement streaming invokes")
	})
	t.Run("Call (unimplemented)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.Call(context.Background(), plugin.CallRequest{})
		assert.ErrorContains(t, err, "the builtin provider does not implement call")
	})
	t.Run("GetPluginInfo (always fails)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.GetPluginInfo(context.Background())
		assert.ErrorContains(t, err, "the builtin provider does not report plugin info")
	})
	t.Run("SignalCancellation", func(t *testing.T) {
		t.Parallel()
		var called bool
		p := &builtinProvider{
			cancel: func() {
				called = true
			},
		}
		assert.NoError(t, p.SignalCancellation(context.Background()))
		assert.True(t, called)
		// Ensure idempotent.
		assert.NoError(t, p.SignalCancellation(context.Background()))
	})
}
