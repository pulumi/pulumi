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

package lifecycletest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/blang/semver"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
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

func TransformInvokeFunction(
	f func(
		token string, props resource.PropertyMap, opts *pulumirpc.TransformInvokeOptions,
	) (resource.PropertyMap, *pulumirpc.TransformInvokeOptions, error),
) func([]byte) (proto.Message, error) {
	return func(request []byte) (proto.Message, error) {
		var transformationRequest pulumirpc.TransformInvokeRequest
		err := proto.Unmarshal(request, &transformationRequest)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling request: %w", err)
		}

		margs, err := plugin.UnmarshalProperties(transformationRequest.Args, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, fmt.Errorf("unmarshaling properties: %w", err)
		}

		ret, opts, err := f(
			transformationRequest.Token, margs, transformationRequest.Options)
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

		return &pulumirpc.TransformInvokeResponse{
			Args:    mret,
			Options: opts,
		}, nil
	}
}

func pvApply(pv resource.PropertyValue, f func(resource.PropertyValue) resource.PropertyValue) resource.PropertyValue {
	if pv.IsOutput() {
		o := pv.OutputValue()
		if !o.Known {
			return pv
		}
		return resource.NewOutputProperty(resource.Output{
			Element:      f(o.Element),
			Known:        true,
			Secret:       o.Secret,
			Dependencies: o.Dependencies,
		})
	}
	return f(pv)
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
				props["foo"] = pvApply(props["foo"], func(v resource.PropertyValue) resource.PropertyValue {
					return resource.NewNumberProperty(v.NumberValue() + 1)
				})
				// callback 2 should run before this one so "bar" should exist at this point
				props["bar"] = resource.NewStringProperty(props["bar"].StringValue() + "baz")

				return props, opts, nil
			}))
		require.NoError(t, err)

		callback2, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				props["foo"] = pvApply(props["foo"], func(v resource.PropertyValue) resource.PropertyValue {
					return resource.NewNumberProperty(v.NumberValue() + 1)
				})
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
				props["foo"] = pvApply(props["foo"], func(v resource.PropertyValue) resource.PropertyValue {
					return resource.NewNumberProperty(v.NumberValue() + 1)
				})
				props["frob"] = resource.NewStringProperty("frob")
				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
			Transforms: []*pulumirpc.Callback{
				callback2,
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(10),
			},
			Transforms: []*pulumirpc.Callback{
				callback3,
			},
			Parent: respA.URN,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
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

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
		})
		assert.ErrorContains(t, err, "unmarshaling response: proto:")
		assert.ErrorContains(t, err, "cannot parse invalid wire-format data")
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.ErrorContains(t, err, "unmarshaling response: proto:")
	assert.ErrorContains(t, err, "cannot parse invalid wire-format data")
	assert.Len(t, snap.Resources, 0)
}

// Test that the engine errors if a transformation function returns an error.
func TestRemoteTransformErrorResponse(t *testing.T) {
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
			return nil, errors.New("bad transform")
		})
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
		})
		assert.ErrorContains(t, err, "Unknown desc = bad transform")
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.ErrorContains(t, err, "Unknown desc = bad transform")
	assert.Len(t, snap.Resources, 0)
}

// Test that a remote transform applies to a resource inside a component construct.
func TestRemoteTransformationsConstruct(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					assert.Equal(t, "pkgA:m:typC", string(req.Type))

					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
					require.NoError(t, err)

					_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
						Parent: resp.URN,
						Inputs: resource.PropertyMap{
							"foo": resource.NewNumberProperty(1),
						},
					})
					require.NoError(t, err)

					return plugin.ConstructResponse{
						URN: resp.URN,
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
					assert.Equal(t, "urn:pulumi:test::test::pkgA:m:typC::resC", parent)
					props["foo"] = pvApply(props["foo"], func(v resource.PropertyValue) resource.PropertyValue {
						return resource.NewNumberProperty(v.NumberValue() + 1)
					})
				}
				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typC", "resC", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
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

	urnB := "urn:pulumi:test::test::pkgA:m:typA::resB"
	urnC := "urn:pulumi:test::test::pkgA:m:typA::resC"
	urnD := "urn:pulumi:test::test::pkgA:m:typA::resD"

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Version: "1.0.0",
		})
		require.NoError(t, err)

		callback1, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				// Check that the options are passed through correctly
				assert.Equal(t, []string{"foo"}, opts.AdditionalSecretOutputs)
				assert.Equal(t, urnB, opts.Aliases[0].Alias.(*pulumirpc.Alias_Urn).Urn)
				assert.Equal(t, "16m40s", opts.CustomTimeouts.Create)
				assert.Equal(t, "33m20s", opts.CustomTimeouts.Update)
				assert.Equal(t, "50m0s", opts.CustomTimeouts.Delete)
				assert.True(t, *opts.DeleteBeforeReplace)
				assert.Equal(t, string(respA.URN), opts.DeletedWith)
				assert.Equal(t, []string{string(respA.URN)}, opts.DependsOn)
				assert.Equal(t, []string{"foo"}, opts.IgnoreChanges)
				assert.Equal(t, "http://server", opts.PluginDownloadUrl)
				assert.Nil(t, opts.Protect)
				assert.Equal(t, []string{"foo"}, opts.ReplaceOnChanges)
				assert.Equal(t, "2.0.0", opts.Version)

				// Modify all the options
				protect := true
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
					DeletedWith:         string(respC.URN),
					DependsOn:           []string{string(respC.URN)},
					IgnoreChanges:       []string{"bar"},
					PluginDownloadUrl:   "",
					Protect:             &protect,
					ReplaceOnChanges:    []string{"bar"},
					Version:             "1.0.0",
				}

				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback1)
		require.NoError(t, err)

		dbr := true
		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			AdditionalSecretOutputs: []resource.PropertyKey{"foo"},
			Aliases: []*pulumirpc.Alias{
				{Alias: &pulumirpc.Alias_Urn{Urn: urnB}},
			},
			CustomTimeouts: &resource.CustomTimeouts{
				Create: 1000,
				Update: 2000,
				Delete: 3000,
			},
			DeleteBeforeReplace: &dbr,
			DeletedWith:         respA.URN,
			Dependencies:        []resource.URN{respA.URN},
			IgnoreChanges:       []string{"foo"},
			PluginDownloadURL:   "http://server",
			ReplaceOnChanges:    []string{"foo"},
			Version:             "2.0.0",
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 5)
	// Check Resources[4] is the resD resource
	res := snap.Resources[4]
	require.Equal(t, resource.URN(urnD), res.URN)
	assert.Equal(t, []resource.PropertyKey{"bar"}, res.AdditionalSecretOutputs)
	assert.Equal(t, resource.CustomTimeouts{
		Create: 1,
		Update: 2,
		Delete: 3,
	}, res.CustomTimeouts)
	assert.Equal(t, resource.URN(urnC), res.DeletedWith)
	assert.Equal(t, true, res.Protect)
}

// Test that a transform can change the dependencies of a resource.
func TestRemoteTransformsDependencies(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "some-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(1),
			},
		})
		require.NoError(t, err)
		assert.True(t, respA.Outputs["foo"].IsNumber())

		// Register a separate resource that
		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewNumberProperty(10),
			},
		})
		require.NoError(t, err)

		callback, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				// props should be tracking that it depends on resB
				assert.True(t, props["foo"].IsOutput())
				assert.Equal(t, []resource.URN{respB.URN}, props["foo"].OutputValue().Dependencies)

				// Add a dependency on resA
				props["foo"] = resource.NewOutputProperty(resource.Output{
					Element:      respA.Outputs["foo"],
					Known:        true,
					Dependencies: []resource.URN{respA.URN},
				})

				return props, opts, nil
			}))
		require.NoError(t, err)

		// Register a resource that initially depends on resB but the transform will turn to depend on resA
		respC, err := monitor.RegisterResource(
			"pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"foo": respB.Outputs["foo"],
				},
				PropertyDeps: map[resource.PropertyKey][]resource.URN{
					"foo": {respB.URN},
				},
				Transforms: []*pulumirpc.Callback{
					callback,
				},
			})
		require.NoError(t, err)
		assert.True(t, respC.Outputs["foo"].IsNumber())
		// This is a custom resource so no output dependencies
		assert.Empty(t, respC.Dependencies)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.Len(t, snap.Resources, 4)
	// Check Resources[3] is the resC resource
	res := snap.Resources[3]
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resC"), res.URN)
	// Check it's final input properties match what we expected from the transformations
	assert.Equal(t, resource.PropertyMap{
		"foo": resource.NewNumberProperty(1),
	}, res.Inputs)
	// Check the dependencies are as expected
	assert.Equal(t, map[resource.PropertyKey][]resource.URN{
		"foo": {resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA")},
	}, res.PropertyDependencies)
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::resA",
	}, res.Dependencies)
}

// Regression test for https://github.com/pulumi/pulumi/issues/15843. Ensure that if a component resource has a
// transform that's saved and looked up by it's children.
func TestRemoteComponentTransforms(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					assert.Equal(t, "pkgA:m:typC", string(req.Type))

					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
					require.NoError(t, err)

					_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
						Parent: resp.URN,
						Inputs: resource.PropertyMap{
							"foo": resource.NewNumberProperty(1),
						},
					})
					require.NoError(t, err)

					return plugin.ConstructResponse{
						URN: resp.URN,
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
					props["foo"] = pvApply(props["foo"], func(v resource.PropertyValue) resource.PropertyValue {
						return resource.NewNumberProperty(v.NumberValue() + 1)
					})
				}
				return props, opts, nil
			}))
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typC", "resC", false, deploytest.ResourceOptions{
			Remote: true,
			Transforms: []*pulumirpc.Callback{
				callback1,
			},
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
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

func TestTransformsProviderOpt(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				Package: "pkgA",
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "some-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var explicitProvider string
	var implicitProvider string
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:providers:pkgA", "explicit", true)
		require.NoError(t, err)
		explicitProvider = string(resp.URN) + "::" + resp.ID.String()

		resp, err = monitor.RegisterResource("pulumi:providers:pkgA", "implicit", true)
		require.NoError(t, err)
		implicitProvider = string(resp.URN) + "::" + resp.ID.String()

		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		callback, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				if opts.Provider == "" {
					opts.Provider = implicitProvider
				}

				return props, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackTransform(callback)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "explicitProvider", true, deploytest.ResourceOptions{
			Provider: explicitProvider,
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "implicitProvider", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "explicitProvidersMap", true, deploytest.ResourceOptions{
			Providers: map[string]string{"pkgA": explicitProvider},
		})
		require.NoError(t, err)

		resp, err = monitor.RegisterResource("xmy:component:resource", "component", false, deploytest.ResourceOptions{
			Providers: map[string]string{"pkgA": explicitProvider},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "parentedResource", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		require.NoError(t, err)

		resp, err = monitor.RegisterResource("ymy:component:resource", "another-component", false, deploytest.ResourceOptions{
			Provider: explicitProvider,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "parentedResource", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{
			{
				Op: Update,
			},
		},
	}
	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Equal(t, 9, len(snap.Resources)) // 2 providers + 7 resources
	sort.Slice(snap.Resources, func(i, j int) bool {
		return snap.Resources[i].URN < snap.Resources[j].URN
	})
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pkgA:m:typA::explicitProvider"), snap.Resources[0].URN)
	assert.Equal(t, explicitProvider, snap.Resources[0].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pkgA:m:typA::explicitProvidersMap"), snap.Resources[1].URN)
	assert.Equal(t, explicitProvider, snap.Resources[1].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pkgA:m:typA::implicitProvider"), snap.Resources[2].URN)
	assert.Equal(t, implicitProvider, snap.Resources[2].Provider)
	assert.Equal(t,
		urn.URN("urn:pulumi:test::test::xmy:component:resource$pkgA:m:typA::parentedResource"),
		snap.Resources[5].URN)
	assert.Equal(t, explicitProvider, snap.Resources[5].Provider)
	assert.Equal(t,
		urn.URN("urn:pulumi:test::test::ymy:component:resource$pkgA:m:typA::parentedResource"),
		snap.Resources[7].URN)
	assert.Equal(t, implicitProvider, snap.Resources[7].Provider)
}

func TestTransformInvoke(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				Package: "pkgA",
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{Properties: req.Args}, nil
				},
			}, nil
		}),
	}

	var implicitProvider string
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:providers:pkgA", "implicit", true)
		require.NoError(t, err)
		implicitProvider = string(resp.URN) + "::" + resp.ID.String()

		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		callback, err := callbacks.Allocate(
			TransformInvokeFunction(func(token string,
				args resource.PropertyMap, opts *pulumirpc.TransformInvokeOptions,
			) (resource.PropertyMap, *pulumirpc.TransformInvokeOptions, error) {
				args["foo"] = resource.NewStringProperty("bar")

				return args, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackInvokeTransform(callback)
		require.NoError(t, err)

		input := resource.PropertyMap{
			"foo": resource.NewStringProperty("baz"),
			"bar": resource.NewStringProperty("qux"),
		}

		result, _, err := monitor.Invoke("pkgA:m:typA", input, implicitProvider, "0.0.0", "")
		require.NoError(t, err)

		assert.Equal(t, "bar", result["foo"].StringValue())
		assert.Equal(t, "qux", result["bar"].StringValue())
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{
			{
				Op: Update,
			},
		},
	}
	_ = p.Run(t, nil)
}

func TestTransformInvokeTransformProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				Package: "pkgA",
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{Properties: req.Args}, nil
				},
			}, nil
		}),
	}

	var implicitProvider string
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:providers:pkgA", "implicit", true)
		require.NoError(t, err)
		implicitProvider = string(resp.URN) + "::" + resp.ID.String()

		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		callback, err := callbacks.Allocate(
			TransformInvokeFunction(func(token string,
				args resource.PropertyMap, opts *pulumirpc.TransformInvokeOptions,
			) (resource.PropertyMap, *pulumirpc.TransformInvokeOptions, error) {
				if opts.Provider == "" {
					opts.Provider = implicitProvider
				}

				return args, opts, nil
			}))
		require.NoError(t, err)

		err = monitor.RegisterStackInvokeTransform(callback)
		require.NoError(t, err)

		input := resource.PropertyMap{}

		_, _, err = monitor.Invoke("pkgA:m:typA", input, "", "", "")
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps: []lt.TestStep{
			{
				Op: Update,
			},
		},
	}
	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Equal(t, 1, len(snap.Resources)) // expect no default provider to be created for the invoke
}
