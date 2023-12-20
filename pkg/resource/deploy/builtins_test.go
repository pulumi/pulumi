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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
		b, err := p.GetSchema(0)
		assert.NoError(t, err)
		assert.Equal(t, []byte("{}"), b)
	})
	t.Run("GetMapping", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		b, s, err := p.GetMapping("key", "provider")
		assert.NoError(t, err)
		assert.Nil(t, b)
		assert.Equal(t, "", s)
	})
	t.Run("GetMappings", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		strs, err := p.GetMappings("key")
		assert.NoError(t, err)
		assert.Empty(t, strs)
	})
	t.Run("Check", func(t *testing.T) {
		t.Parallel()
		t.Run("builtin only supports stack reference type", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{}
			_, _, err := p.Check(
				resource.CreateURN("foo", "not-stack-reference-type", "", "proj", "stack"),
				resource.PropertyMap{}, resource.PropertyMap{}, true, nil)
			assert.ErrorContains(t, err, "unrecognized resource type")
		})
		t.Run("missing `name` input property", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			_, failures, err := p.Check(
				resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				resource.PropertyMap{}, resource.PropertyMap{}, true, nil)
			assert.Equal(t, []plugin.CheckFailure{
				{
					Property: "name",
					Reason:   `missing required property "name"`,
				},
			}, failures)
			assert.NoError(t, err)
		})
		t.Run(`property "name" must be a string`, func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			_, failures, err := p.Check(
				resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				resource.PropertyMap{}, resource.PropertyMap{
					"name": resource.NewNumberProperty(10),
				}, true, nil)
			assert.Equal(t, []plugin.CheckFailure{
				{
					Property: "name",
					Reason:   `property "name" must be a string`,
				},
			}, failures)
			assert.NoError(t, err)
		})
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			p := &builtinProvider{
				diag: &deploytest.NoopSink{},
			}
			checked, failures, err := p.Check(
				resource.CreateURN("foo", stackReferenceType, "", "proj", "stack"),
				resource.PropertyMap{}, resource.PropertyMap{
					"name": resource.NewStringProperty("res-name"),
				}, true, nil)
			assert.Nil(t, failures)
			assert.NoError(t, err)
			assert.Equal(t, resource.PropertyMap{
				"name": resource.NewStringProperty("res-name"),
			}, checked)
		})
	})
	t.Run("Update (always fails)", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			p := &builtinProvider{}

			oldOutputs := resource.PropertyMap{"cookie": resource.NewStringProperty("yum")}
			_, _, err := p.Update(
				resource.CreateURN("foo", "not-stack-reference-type", "", "proj", "stack"),
				"some-id",
				nil, oldOutputs,
				resource.PropertyMap{}, 0, nil, false,
			)
			contract.Ignore(err)
		})
	})
	t.Run("Construct (always fails)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.Construct(plugin.ConstructInfo{}, "", "", "", resource.PropertyMap{}, plugin.ConstructOptions{})
		assert.ErrorContains(t, err, "builtin resources may not be constructed")
	})
	t.Run("Invoke", func(t *testing.T) {
		t.Parallel()
		t.Run(readStackOutputs, func(t *testing.T) {
			t.Parallel()
			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{}
				_, _, err := p.Invoke(readStackOutputs, resource.PropertyMap{
					"name": resource.NewStringProperty("res-name"),
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
				out, failures, err := p.Invoke(readStackOutputs, resource.PropertyMap{
					"name": resource.NewStringProperty("res-name"),
				})
				assert.NoError(t, err)
				assert.True(t, called)
				assert.Nil(t, failures)

				assert.Equal(t, "res-name", out["name"].V)

				assert.Equal(t, "foo", out["outputs"].ObjectValue()["normal"].StringValue())
				assert.Len(t, out["secretOutputNames"].V, 1)
			})
		})
		t.Run(readStackResourceOutputs, func(t *testing.T) {
			t.Parallel()
			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{}
				_, _, err := p.Invoke(readStackResourceOutputs, resource.PropertyMap{
					"stackName": resource.NewStringProperty("res-name"),
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
				_, _, err := p.Invoke(readStackResourceOutputs, resource.PropertyMap{
					"stackName": resource.NewStringProperty("res-name"),
				})
				assert.NoError(t, err)
				assert.True(t, called)
			})
		})
		t.Run(getResource, func(t *testing.T) {
			t.Parallel()
			t.Run("err", func(t *testing.T) {
				t.Parallel()
				p := &builtinProvider{
					resources: &resourceMap{},
				}
				_, _, err := p.Invoke(getResource, resource.PropertyMap{
					"urn": resource.NewStringProperty("res-name"),
				})
				assert.ErrorContains(t, err, "unknown resource")
			})
		})
	})
	t.Run("StreamInvoke (unimplemented)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.StreamInvoke(tokens.ModuleMember(""), resource.PropertyMap{}, nil)
		assert.ErrorContains(t, err, "the builtin provider does not implement streaming invokes")
	})
	t.Run("Call (unimplemented)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.Call(tokens.ModuleMember(""), resource.PropertyMap{},
			plugin.CallInfo{}, plugin.CallOptions{})
		assert.ErrorContains(t, err, "the builtin provider does not implement call")
	})
	t.Run("GetPluginInfo (always fails)", func(t *testing.T) {
		t.Parallel()
		p := &builtinProvider{}
		_, err := p.GetPluginInfo()
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
		assert.NoError(t, p.SignalCancellation())
		assert.True(t, called)
		// Ensure idempotent.
		assert.NoError(t, p.SignalCancellation())
	})
}
