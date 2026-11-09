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

package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Tests that when a RegisterResourceRequest sets uses_output_values=true the engine harvests per-leaf
// propertyDependencies from the OutputValues on the inputs rather than looking at the (empty)
// propertyDependencies map on the wire.
func TestRegisterUsesOutputValues(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"value": resource.NewProperty("a")},
		})
		require.NoError(t, err)
		respB, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"value": resource.NewProperty("b")},
		})
		require.NoError(t, err)

		// Register `child` using the uses_output_values wire shape: inputs carry per-leaf OutputValue
		// deps and PropertyDeps is nil. The engine should derive child.PropertyDependencies from the OVs.
		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty(resource.Output{
					Element:      resource.NewProperty("a"),
					Known:        true,
					Dependencies: []resource.URN{respA.URN},
				}),
				"bar": resource.NewProperty(resource.Output{
					Element:      resource.NewProperty("b"),
					Known:        true,
					Dependencies: []resource.URN{respB.URN},
				}),
			},
			UsesOutputValues: true,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()
	snap, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	var a, b, child *pkgresource.State
	for _, r := range snap.Resources {
		switch r.URN.Name() {
		case "a":
			a = r
		case "b":
			b = r
		case "child":
			child = r
		}
	}
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.NotNil(t, child)

	assert.Equal(t, []resource.URN{a.URN}, child.PropertyDependencies["foo"])
	assert.Equal(t, []resource.URN{b.URN}, child.PropertyDependencies["bar"])
	// child.Dependencies reflects only explicit depends_on, which is empty here. Engine consumers that
	// want the combined set go through State.GetAllDependencies().
	assert.Empty(t, child.Dependencies)
	// Inputs should be downgraded to plain values before being persisted to state (step_generator
	// does not yet understand OutputValues).
	assert.Equal(t, resource.PropertyMap{
		"foo": resource.NewProperty("a"),
		"bar": resource.NewProperty("b"),
	}, child.Inputs)
}

// Tests that a resource transform registered on a uses_output_values RegisterResource receives the
// per-leaf OutputValues (with dependencies) nested inside a top-level property, and that lifting those
// leaves to new top-level properties preserves per-leaf dependency attribution.
//
// This case cannot be expressed by the legacy per-top-level-field propertyDependencies wire shape: the
// child starts with a single top-level input `both` that contains both a- and b-dependent leaves. The
// transform splits them out into new top-level `first` and `second` fields. Only the OV-tree walk can
// attribute `first` to a and `second` to b; the legacy stamping would collapse both leaves of `both`
// (and therefore the lifted fields) to the union {a, b}.
func TestRegisterUsesOutputValuesTransforms(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"value": resource.NewProperty("a")},
		})
		require.NoError(t, err)
		respB, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"value": resource.NewProperty("b")},
		})
		require.NoError(t, err)

		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		// Transform asserts the nested per-leaf OutputValues are visible on `props["both"]`, then lifts
		// each leaf to a new top-level property while leaving `both` in place. After the engine harvests
		// from the modified props the child should carry:
		//   both   -> {a, b}   (both leaves still live here)
		//   first  -> {a}      (lifted from both.x)
		//   second -> {b}      (lifted from both.y)
		split, err := callbacks.Allocate(
			TransformFunction(func(name, typ string, custom bool, parent string,
				props resource.PropertyMap, opts *pulumirpc.TransformResourceOptions,
			) (resource.PropertyMap, *pulumirpc.TransformResourceOptions, error) {
				both := props["both"]
				require.True(t, both.IsObject(), "both should be an object, got %v", both)
				bothMap := both.ObjectValue()

				x, y := bothMap["x"], bothMap["y"]
				require.True(t, x.IsOutput(), "both.x should be an OutputValue, got %v", x)
				require.True(t, y.IsOutput(), "both.y should be an OutputValue, got %v", y)
				assert.Equal(t, []resource.URN{respA.URN}, x.OutputValue().Dependencies,
					"both.x OV should carry only a.URN")
				assert.Equal(t, []resource.URN{respB.URN}, y.OutputValue().Dependencies,
					"both.y OV should carry only b.URN")

				props["first"] = x
				props["second"] = y
				return props, opts, nil
			}))
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"both": resource.NewProperty(resource.PropertyMap{
					"x": resource.NewProperty(resource.Output{
						Element:      resource.NewProperty("a"),
						Known:        true,
						Dependencies: []resource.URN{respA.URN},
					}),
					"y": resource.NewProperty(resource.Output{
						Element:      resource.NewProperty("b"),
						Known:        true,
						Dependencies: []resource.URN{respB.URN},
					}),
				}),
			},
			UsesOutputValues: true,
			Transforms:       []*pulumirpc.Callback{split},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()
	snap, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	var a, b, child *pkgresource.State
	for _, r := range snap.Resources {
		switch r.URN.Name() {
		case "a":
			a = r
		case "b":
			b = r
		case "child":
			child = r
		}
	}
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.NotNil(t, child)

	// `both` keeps both leaves so it depends on both a and b; the lifted leaves attribute precisely.
	assert.ElementsMatch(t, []resource.URN{a.URN, b.URN}, child.PropertyDependencies["both"])
	assert.Equal(t, []resource.URN{a.URN}, child.PropertyDependencies["first"])
	assert.Equal(t, []resource.URN{b.URN}, child.PropertyDependencies["second"])
	// child.Dependencies reflects only explicit depends_on, which is empty here. Engine consumers that
	// want the combined set go through State.GetAllDependencies().
	assert.Empty(t, child.Dependencies)
	assert.Equal(t, resource.PropertyMap{
		"both": resource.NewProperty(resource.PropertyMap{
			"x": resource.NewProperty("a"),
			"y": resource.NewProperty("b"),
		}),
		"first":  resource.NewProperty("a"),
		"second": resource.NewProperty("b"),
	}, child.Inputs)
}

// Tests that the engine rejects a RegisterResourceRequest that sets uses_output_values=true alongside a
// non-empty propertyDependencies map. The two wire shapes are mutually exclusive — SDKs that opt in to
// the OV path must carry per-leaf deps inline on inputs.
func TestRegisterUsesOutputValuesRejectsPropDeps(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{"value": resource.NewProperty("a")},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty(resource.Output{
					Element:      resource.NewProperty("a"),
					Known:        true,
					Dependencies: []resource.URN{respA.URN},
				}),
			},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"foo": {respA.URN},
			},
			UsesOutputValues: true,
		})
		require.Error(t, err) //nolint:forbidigo // pre-existing
		assert.Contains(t, err.Error(), "uses_output_values")
		assert.Contains(t, err.Error(), "propertyDependencies")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()
	// The program returns nil after observing the expected error, so the update itself succeeds.
	_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
}

// Mirrors the l2-component-output-value-input-deps conformance test at the lifecycle level, covering two
// things the conformance test can't easily inspect:
//
//  1. Construct receives the component's inputs as fine-grained OutputValues when the SDK sends
//     uses_output_values=true. The provider's Construct body asserts refs.x and refs.y are both leaf
//     OutputValues carrying their original per-URN dependencies — i.e. the engine forwards the OV tree
//     through to Construct instead of flattening it.
//
//  2. The RegisterResourceResponse that the SDK gets back carries the correct per-output
//     PropertyDependencies regardless of which wire shape Construct used to report them. We run the
//     test twice, once with Construct returning OutputValues inline in Outputs (so the engine harvests
//     per-leaf deps) and once with Construct returning plain outputs plus an explicit OutputDependencies
//     map. In both cases the SDK should observe {out1: [a], out2: [b]}.
func TestRegisterUsesOutputValuesComponentConstruct(t *testing.T) {
	t.Parallel()

	for _, variant := range []struct {
		name                string
		returnOutputValues  bool
		returnOutputDepsMap bool
	}{
		{name: "output_values_in_outputs", returnOutputValues: true, returnOutputDepsMap: false},
		{name: "output_dependencies_map", returnOutputValues: false, returnOutputDepsMap: true},
	} {
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()

			var componentURN resource.URN // captured below for assertions on construct inputs

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{
								ID:         resource.ID("id-" + req.URN.Name()),
								Properties: req.Properties,
								Status:     resource.StatusOK,
							}, nil
						},
						ConstructF: func(
							_ context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
						) (plugin.ConstructResponse, error) {
							// (1) Construct must see fine-grained OutputValues at refs.x and refs.y.
							inputs := resource.ToResourcePropertyMap(req.Inputs)
							refs, ok := inputs["refs"]
							require.True(t, ok, "refs missing from construct inputs: %v", inputs)
							require.True(t, refs.IsObject(), "refs should be an object, got %v", refs)
							refsMap := refs.ObjectValue()

							x, hasX := refsMap["x"]
							y, hasY := refsMap["y"]
							require.True(t, hasX && hasY, "refs.x / refs.y missing: %v", refsMap)
							require.True(t, x.IsOutput(), "refs.x should be an OutputValue, got %v", x)
							require.True(t, y.IsOutput(), "refs.y should be an OutputValue, got %v", y)
							xDeps := x.OutputValue().Dependencies
							yDeps := y.OutputValue().Dependencies

							resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
							require.NoError(t, err)
							componentURN = resp.URN

							// Build the component's outputs in whichever shape this variant exercises. The
							// deps we want to attribute to each output are the same per-leaf URNs we pulled
							// off the inputs — out1 flows from refs.x (depends on a), out2 from refs.y
							// (depends on b).
							var outputs resource.PropertyMap
							var outputDeps map[resource.PropertyKey][]resource.URN
							if variant.returnOutputValues {
								outputs = resource.PropertyMap{
									"out1": resource.NewProperty(resource.Output{
										Element: resource.NewProperty("a-val"), Known: true, Dependencies: xDeps,
									}),
									"out2": resource.NewProperty(resource.Output{
										Element: resource.NewProperty("b-val"), Known: true, Dependencies: yDeps,
									}),
								}
							} else {
								outputs = resource.PropertyMap{
									"out1": resource.NewProperty("a-val"),
									"out2": resource.NewProperty("b-val"),
								}
							}
							if variant.returnOutputDepsMap {
								outputDeps = map[resource.PropertyKey][]resource.URN{
									"out1": xDeps,
									"out2": yDeps,
								}
							}

							return plugin.ConstructResponse{
								URN:                resp.URN,
								Outputs:            resource.FromResourcePropertyMap(outputs),
								OutputDependencies: outputDeps,
							}, nil
						},
					}, nil
				}),
			}

			var observed *deploytest.RegisterResourceResponse
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				respA, err := monitor.RegisterResource("pkgA:m:typA", "a", true, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"value": resource.NewProperty("a")},
				})
				require.NoError(t, err)
				respB, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"value": resource.NewProperty("b")},
				})
				require.NoError(t, err)

				observed, err = monitor.RegisterResource("pkgA:m:typC", "comp", false, deploytest.ResourceOptions{
					Remote: true,
					Inputs: resource.PropertyMap{
						"refs": resource.NewProperty(resource.PropertyMap{
							"x": resource.NewProperty(resource.Output{
								Element: resource.NewProperty("a-val"), Known: true,
								Dependencies: []resource.URN{respA.URN},
							}),
							"y": resource.NewProperty(resource.Output{
								Element: resource.NewProperty("b-val"), Known: true,
								Dependencies: []resource.URN{respB.URN},
							}),
						}),
					},
					UsesOutputValues: true,
				})
				require.NoError(t, err)
				return nil
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

			p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
			project := p.GetProject()
			snap, err := lt.TestOp(engine.Update).Run(
				project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
			require.NoError(t, err)

			var a, b *pkgresource.State
			for _, r := range snap.Resources {
				switch r.URN.Name() {
				case "a":
					a = r
				case "b":
					b = r
				}
			}
			require.NotNil(t, a)
			require.NotNil(t, b)
			require.NotEmpty(t, componentURN, "ConstructF should have been invoked")
			require.NotNil(t, observed)

			// (2) The SDK observes per-output PropertyDependencies regardless of whether Construct
			// reported them via inline OutputValues or an explicit OutputDependencies map.
			assert.Equal(t, []resource.URN{a.URN}, observed.Dependencies["out1"])
			assert.Equal(t, []resource.URN{b.URN}, observed.Dependencies["out2"])
		})
	}
}
