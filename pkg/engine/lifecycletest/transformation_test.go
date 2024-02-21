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

package lifecycletest

import (
	"fmt"
	"testing"

	"github.com/blang/semver"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TransformFunction(
	f func(
		name, typ string, custom bool, parent string,
		props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
	) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error),
) func([]byte) (proto.Message, error) {
	return func(request []byte) (proto.Message, error) {
		var transformationRequest pulumirpc.TransformRequest
		err := proto.Unmarshal(request, &transformationRequest)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling request: %w", err)
		}

		mprops, err := plugin.UnmarshalProperties(transformationRequest.Properties, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, fmt.Errorf("unmarshaling properties: %w", err)
		}

		ret, opts, err := f(
			transformationRequest.Name, transformationRequest.Type, transformationRequest.Custom, transformationRequest.Parent,
			mprops, transformationRequest.Options)
		if err != nil {
			return nil, err
		}
		mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, err
		}

		return &pulumirpc.TransformResponse{
			Properties: mret,
			Options:    opts,
		}, nil
	}
}

// Test that the engine invokes all transformation functions in the correct order.
func TestRemoteTransforms(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback1, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				// callback 2 should run before this one so "bar" should exist at this point
				props["bar"] = resource.NewStringProperty(props["bar"].StringValue() + "baz")

				return props, opts, nil
			}))
		require.NoError(t, err)

		callback2, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				props["bar"] = resource.NewStringProperty("bar")
				// if this is for resB then callback 3 will have run before this one
				if prop, has := props["frob"]; has {
					props["frob"] = resource.MakeSecret(prop)
				} else {
					props["frob"] = resource.NewStringProperty("nofrob")
				}

				return props, opts, nil
			}))
		require.NoError(t, err)

		callback3, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				props["frob"] = resource.NewStringProperty("frob")
				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		aURN, _, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
			Transforms: []*pulumirpc.Callback{
				callback2,
			},
		})
		require.NoError(t, err)

		_, _, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(10),
			},
			Transforms: []*pulumirpc.Callback{
				callback3,
			},
			Parent: aURN,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 3)
	// Check Resources[1] is the resA resource
	res := snap.Resources[1]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo":  resource.NewNumberProperty(3),
		"bar":  resource.NewStringProperty("barbaz"),
		"frob": resource.NewStringProperty("nofrob"),
	}, res.Inputs)

	// Check Resources[2] is the resB resource
	res = snap.Resources[2]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo":  resource.NewNumberProperty(13),
		"bar":  resource.NewStringProperty("barbaz"),
		"frob": resource.MakeSecret(resource.NewStringProperty("frob")),
	}, res.Inputs)
}

// Test that the engine errors if a transformation function returns an unexpected response.
func TestRemoteTransformBadResponse(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback1, err := callbacks.Allocate(func(args []byte) (proto.Message, error) {
			// return the wrong message type
			return &pulumirpc.RegisterResourceResponse{
				Urn: "boom",
			}, nil
		})
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		_, _, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
		})
		assert.ErrorContains(t, err, "unmarshaling response: proto:")
		assert.ErrorContains(t, err, "cannot parse invalid wire-format data")
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.ErrorContains(t, err, "unmarshaling response: proto:")
	assert.ErrorContains(t, err, "cannot parse invalid wire-format data")
	assert.Len(t, snap.Resources, 0)
}

// Test that a remote transform applies to a resource inside a component construct.
func TestRemoteTransformationsConstruct(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					monitor *deploytest.ResourceMonitor, typ string, name string, parent resource.URN,
					inputs resource.PropertyMap, info plugin.ConstructInfo, options plugin.ConstructOptions,
				) (plugin.ConstructResult, error) {
					assert.Equal(t, "pkgA:m:typC", typ)

					urn, _, _, _, err := monitor.RegisterResource(tokens.Type(typ), name, false, deploytest.ResourceOptions{})
					require.NoError(t, err)

					_, _, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
						Parent: urn,
						Inputs: resource.PropertyMap{
							"foo": resource.NewNumberProperty(1),
						},
					})
					require.NoError(t, err)

					return plugin.ConstructResult{
						URN: urn,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback1, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				if typ == "pkgA:m:typA" {
					props["foo"] = resource.NewNumberProperty(props["foo"].NumberValue() + 1)
				}
				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		_, _, _, _, err = monitor.RegisterResource("pkgA:m:typC", "resC", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 3)
	// Check Resources[2] is the resA resource
	res := snap.Resources[2]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resA"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo": resource.NewNumberProperty(2),
	}, res.Inputs)
}

// Test that all options are passed and can be modified by a transformation function.
func TestRemoteTransformsOptions(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	urnA := "urn:pulumi:test::test::pkgA:m:typA::resA"
	urnB := "urn:pulumi:test::test::pkgA:m:typA::resB"
	urnC := "urn:pulumi:test::test::pkgA:m:typA::resC"

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		_, _, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Version: "1.0.0",
		})
		require.NoError(t, err)

		callback1, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				// Check that the options are passed through correctly
				assert.Equal(t, []string{"foo"}, opts.AdditionalSecretOutputs)
				assert.Equal(t, urnA, opts.Aliases[0].Alias.(*pulumirpc.Alias_Urn).Urn)
				assert.Equal(t, "16m40s", opts.CustomTimeouts.Create)
				assert.Equal(t, "33m20s", opts.CustomTimeouts.Update)
				assert.Equal(t, "50m0s", opts.CustomTimeouts.Delete)
				assert.True(t, *opts.DeleteBeforeReplace)
				assert.Equal(t, urnB, opts.DeletedWith)
				assert.Equal(t, []string{urnB}, opts.DependsOn)
				assert.Equal(t, []string{"foo"}, opts.IgnoreChanges)
				assert.Equal(t, "http://server", opts.PluginDownloadUrl)
				assert.Equal(t, false, opts.Protect)
				assert.Equal(t, []string{"foo"}, opts.ReplaceOnChanges)
				assert.Equal(t, "2.0.0", opts.Version)

				// Modify all the options
				opts = &pulumirpc.TransformResourceOptions{
					AdditionalSecretOutputs: []string{"bar"},
					Aliases: []*pulumirpc.Alias{
						{Alias: &pulumirpc.Alias_Urn{Urn: urnB}},
					},
					CustomTimeouts: &pulumirpc.RegisterResourceRequest_CustomTimeouts{
						Create: "1s",
						Update: "2s",
						Delete: "3s",
					},
					DeleteBeforeReplace: nil,
					DeletedWith:         urnC,
					DependsOn:           []string{urnC},
					IgnoreChanges:       []string{"bar"},
					PluginDownloadUrl:   "",
					Protect:             true,
					ReplaceOnChanges:    []string{"bar"},
					Version:             "1.0.0",
				}

				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		dbr := true
		_, _, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			AdditionalSecretOutputs: []resource.PropertyKey{"foo"},
			Aliases: []*pulumirpc.Alias{
				{Alias: &pulumirpc.Alias_Urn{Urn: urnA}},
			},
			CustomTimeouts: &resource.CustomTimeouts{
				Create: 1000,
				Update: 2000,
				Delete: 3000,
			},
			DeleteBeforeReplace: &dbr,
			DeletedWith:         resource.URN(urnB),
			Dependencies:        []resource.URN{resource.URN(urnB)},
			IgnoreChanges:       []string{"foo"},
			PluginDownloadURL:   "http://server",
			ReplaceOnChanges:    []string{"foo"},
			Version:             "2.0.0",
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
	// Check Resources[2] is the resA resource
	res := snap.Resources[2]
	require.Equal(t, resource.URN(urnA), res.URN)
	assert.Equal(t, []resource.PropertyKey{"bar"}, res.AdditionalSecretOutputs)
	assert.Equal(t, resource.CustomTimeouts{
		Create: 1,
		Update: 2,
		Delete: 3,
	}, res.CustomTimeouts)
	assert.Equal(t, resource.URN(urnC), res.DeletedWith)
	assert.Equal(t, true, res.Protect)
}
