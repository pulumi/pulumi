// Copyright 2024-2024, Pulumi Corporation.
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
	"fmt"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestPackageRef tests we can request a package ref from the engine and then use that, instead of Version,
// PackageDownloadURL etc.
func TestPackageRef(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "0",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "1",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkg1Ref, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil)
		require.NoError(t, err)
		pkg2Ref, err := monitor.RegisterPackage("pkgA", "2.0.0", "", nil, nil)
		require.NoError(t, err)

		// If we register the "same" provider in parallel, we should get the same ref.
		promises := []*promise.Promise[string]{}
		for i := 0; i < 100; i++ {
			var pcs promise.CompletionSource[string]
			promises = append(promises, pcs.Promise())
			go func() {
				ref, err := monitor.RegisterPackage("pkgB", "1.0.0", "downloadUrl", nil, nil)
				require.NoError(t, err)
				pcs.MustFulfill(ref)
			}()
		}
		ctx := context.Background()
		expected, err := promises[0].Result(ctx)
		require.NoError(t, err)
		for i := 1; i < 100; i++ {
			got, err := promises[i].Result(ctx)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		}

		// Now register some resources using the UUID for the provider, instead of a normal provider ref.
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkg1Ref,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("0"), resp.ID)

		resp, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: pkg2Ref,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("1"), resp.ID)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)

	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, string(snap.Resources[0].URN)+"::"+string(snap.Resources[0].ID), snap.Resources[1].Provider)
	assert.Equal(t, string(snap.Resources[2].URN)+"::"+string(snap.Resources[2].ID), snap.Resources[3].Provider)
}

// TestReplacementParameterizedProvider tests that we can register a parameterized provider that replaces a base
// provider.
func TestReplacementParameterizedProvider(t *testing.T) {
	t.Parallel()

	loadCount := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			loadCount++

			var param string

			return &deploytest.Provider{
				ParameterizeF: func(
					ctx context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)

					param = string(value.Value)

					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Type() == "pkgExt:m:typA" {
						assert.Equal(t, "replacement", param)
					}

					return plugin.CreateResponse{
						ID:         "id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, "pkgExt:index:func", req.Tok.String())
					assert.Equal(t, resource.NewStringProperty("in"), req.Args["input"])

					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"output": resource.NewStringProperty("in " + param),
						},
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if param == "" {
						assert.Equal(t, tokens.Type("pkgA:m:typA"), req.URN.Type())
					} else {
						assert.Equal(t, tokens.Type("pkgExt:m:typA"), req.URN.Type())
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
				CallF: func(_ context.Context, req plugin.CallRequest, _ *deploytest.ResourceMonitor) (plugin.CallResponse, error) {
					assert.Equal(t, "pkgExt:index:call", req.Tok.String())
					assert.Equal(t, resource.NewStringProperty("in"), req.Args["input"])
					assert.Equal(t, map[resource.PropertyKey][]resource.URN{
						"input": {"urn:pulumi:stack::m::typA::resB"},
					}, req.Options.ArgDependencies)

					return plugin.CallResponse{
						Return: resource.PropertyMap{
							"output": resource.NewStringProperty("output"),
						},
						ReturnDependencies: map[resource.PropertyKey][]resource.URN{
							"output": {"urn:pulumi:stack::m::typA::resB"},
						},
						Failures: nil,
					}, nil
				},
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if param == "" {
						assert.Equal(t, tokens.Type("pkgA:m:typA"), req.Type)
						assert.Equal(t, "mlcA", req.Name)
					} else {
						assert.Equal(t, tokens.Type("pkgExt:m:typA"), req.Type)
						assert.Equal(t, "mlcB", req.Name)
					}

					return plugin.ConstructResponse{
						URN: resource.NewURN("", "", "", req.Type, req.Name),
						Outputs: resource.PropertyMap{
							"output": resource.NewStringProperty("output"),
						},
						OutputDependencies: map[resource.PropertyKey][]resource.URN{
							"output": {"urn:pulumi:stack::m::typA::resB"},
						},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil)
		require.NoError(t, err)

		// Register a resource using that base provider
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkgRef,
		})
		require.NoError(t, err)

		// Register a multi-language component with the base provider
		mlcA, err := monitor.RegisterResource("pkgA:m:typA", "mlcA", true, deploytest.ResourceOptions{
			PackageRef: pkgRef,
			Remote:     true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "mlcA", mlcA.URN.Name())
		assert.Equal(t, resource.PropertyMap{
			"output": resource.NewStringProperty("output"),
		}, mlcA.Outputs)
		assert.Equal(t, map[resource.PropertyKey][]resource.URN{
			"output": {"urn:pulumi:stack::m::typA::resB"},
		}, mlcA.Dependencies)

		// Now register a replacement provider
		extRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, &pulumirpc.Parameterization{
			Name:    "pkgExt",
			Version: "0.5.0",
			Value:   []byte("replacement"),
		})
		require.NoError(t, err)

		// Test registering a resource with the replacement provider
		_, err = monitor.RegisterResource("pkgExt:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: extRef,
		})
		require.NoError(t, err)

		// Register a multi-language component with the replacement provider
		mlcB, err := monitor.RegisterResource("pkgExt:m:typA", "mlcB", true, deploytest.ResourceOptions{
			PackageRef: extRef,
			Remote:     true,
		})
		assert.NoError(t, err)
		assert.Equal(t, "mlcB", mlcB.URN.Name())
		assert.Equal(t, resource.PropertyMap{
			"output": resource.NewStringProperty("output"),
		}, mlcB.Outputs)
		assert.Equal(t, map[resource.PropertyKey][]resource.URN{
			"output": {"urn:pulumi:stack::m::typA::resB"},
		}, mlcB.Dependencies)

		// Test invoking a function on the replacement provider
		result, _, err := monitor.Invoke("pkgExt:index:func", resource.PropertyMap{
			"input": resource.NewStringProperty("in"),
		}, "", "", extRef)
		require.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"output": resource.NewStringProperty("in replacement"),
		}, result)

		// Test reading a resource on the replacement provider
		_, _, err = monitor.ReadResource("pkgExt:m:typA", "resC", "id", "", resource.PropertyMap{}, "", "", "", extRef)
		require.NoError(t, err)

		// Test calling a function on the replacement provider
		callOuts, callDeps, callFailures, err := monitor.Call(
			"pkgExt:index:call",
			resource.PropertyMap{
				"input": resource.NewStringProperty("in"),
			},
			map[resource.PropertyKey][]resource.URN{
				"input": {"urn:pulumi:stack::m::typA::resB"},
			},
			"", /*provider*/
			"", /*version*/
			extRef,
		)
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"output": resource.NewStringProperty("output"),
		}, callOuts)
		assert.Equal(t, map[resource.PropertyKey][]resource.URN{
			"output": {"urn:pulumi:stack::m::typA::resB"},
		}, callDeps)
		assert.Nil(t, callFailures)

		// Test that we can create an explicit replacement provider and can use it
		prov, err := monitor.RegisterResource("pulumi:providers:pkgExt", "provider", true, deploytest.ResourceOptions{
			PackageRef: extRef,
		})
		assert.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgExt:m:typA", "resD", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		require.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "up")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 7)

	// Check that we loaded the provider thrice
	assert.Equal(t, 3, loadCount)

	// Check the state of the parameterized provider is what we expect
	prov := snap.Resources[2]
	assert.Equal(t, tokens.Type("pulumi:providers:pkgExt"), prov.Type)
	assert.Equal(t, "default_0_5_0", prov.URN.Name())
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]any{
		"version": "0.5.0",
		"__internal": map[string]any{
			"name":             "pkgA",
			"version":          "1.0.0",
			"parameterization": "cmVwbGFjZW1lbnQ=",
		},
	}), prov.Inputs)

	snap, err = lt.TestOp(Refresh).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "refresh")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 7)

	snap, err = lt.TestOp(Destroy).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "destroy")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

// TestReplacementParameterizedProviderConfig tests that we can register a parameterized provider that uses config keys
// like "name" without clashing against the internal state the engine tracks for parameterization. c.f.
// https://github.com/pulumi/pulumi/issues/16757.
func TestReplacementParameterizedProviderConfig(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			var param string
			return &deploytest.Provider{
				ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					// Ensure that the provider configuration is what we expect.
					var expected resource.PropertyMap
					if param == "replacement" {
						expected = resource.NewPropertyMapFromMap(map[string]any{
							"version": "0.5.0",
							"name":    "testingExt",
						})
					} else {
						expected = resource.NewPropertyMapFromMap(map[string]any{
							"version": "1.0.0",
							"name":    "testingBase",
						})
					}

					if !req.Inputs.DeepEquals(expected) {
						return plugin.ConfigureResponse{},
							fmt.Errorf("expected provider configuration to be %v, got %v", expected, req.Inputs)
					}
					return plugin.ConfigureResponse{}, nil
				},
				ParameterizeF: func(
					ctx context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)

					param = string(value.Value)

					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Type() == "pkgExt:m:typA" {
						assert.Equal(t, "replacement", param)
					}

					return plugin.CreateResponse{
						ID:         "id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "http://example.com", nil, nil)
		require.NoError(t, err)

		// Register a resource using that base provider
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkgRef,
		})
		require.NoError(t, err)

		// Now register a replacement provider
		extRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, &pulumirpc.Parameterization{
			Name:    "pkgExt",
			Version: "0.5.0",
			Value:   []byte("replacement"),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgExt:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: extRef,
		})
		require.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Config: config.Map{
			config.MustParseKey("pkgA:name"):   config.NewValue("testingBase"),
			config.MustParseKey("pkgExt:name"): config.NewValue("testingExt"),
		},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "up")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)

	// Check the state of the parameterized provider is what we expect
	prov := snap.Resources[2]
	assert.Equal(t, tokens.Type("pulumi:providers:pkgExt"), prov.Type)
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]any{
		"version": "0.5.0",
		"name":    "testingExt",
		"__internal": map[string]any{
			"name":             "pkgA",
			"version":          "1.0.0",
			"parameterization": "cmVwbGFjZW1lbnQ=",
		},
	}), prov.Inputs)

	snap, err = lt.TestOp(Refresh).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "refresh")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)

	snap, err = lt.TestOp(Destroy).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "destroy")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

// TestReplacementParameterizedProviderImport tests that we can register a parameterized provider that replaces a base
// provider and use it to import resources. Test with both default and explicit providers
func TestReplacementParameterizedProviderImport(t *testing.T) {
	t.Parallel()

	loadCount := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			loadCount++

			var param string

			return &deploytest.Provider{
				ParameterizeF: func(
					ctx context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)

					param = string(value.Value)

					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if param == "" {
						assert.Equal(t, tokens.Type("pkgA:m:typA"), req.URN.Type())
						assert.Equal(t, resource.ID("idA"), req.ID)
					} else {
						assert.Equal(t, tokens.Type("pkgExt:m:typA"), req.URN.Type())
						assert.Equal(t, resource.ID("idB"), req.ID)
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID: req.ID,
							Inputs: resource.PropertyMap{
								"input": resource.NewStringProperty("input"),
							},
							Outputs: resource.PropertyMap{
								"output": resource.NewStringProperty("output"),
							},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil)
		require.NoError(t, err)

		// Import a resource using that base provider
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			PackageRef: pkgRef,
			ImportID:   "idA",
			Inputs: resource.PropertyMap{
				"input": resource.NewStringProperty("input"),
			},
		})
		require.NoError(t, err)

		// Now register a replacement provider
		extRef, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, &pulumirpc.Parameterization{
			Name:    "pkgExt",
			Version: "0.5.0",
			Value:   []byte("replacement"),
		})
		require.NoError(t, err)

		// Test importing a resource with the replacement provider
		_, err = monitor.RegisterResource("pkgExt:m:typA", "resB", true, deploytest.ResourceOptions{
			PackageRef: extRef,
			ImportID:   "idB",
			Inputs: resource.PropertyMap{
				"input": resource.NewStringProperty("input"),
			},
		})
		require.NoError(t, err)

		// Test that we can create an explicit replacement provider and can use it
		prov, err := monitor.RegisterResource("pulumi:providers:pkgExt", "provider", true, deploytest.ResourceOptions{
			PackageRef: extRef,
		})
		assert.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgExt:m:typA", "resC", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			ImportID: "idB",
			Inputs: resource.PropertyMap{
				"input": resource.NewStringProperty("input"),
			},
		})
		require.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "up")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 6)

	// Check that we loaded the provider thrice
	assert.Equal(t, 3, loadCount)

	// Check the state of the parameterized provider is what we expect
	prov := snap.Resources[2]
	assert.Equal(t, tokens.Type("pulumi:providers:pkgExt"), prov.Type)
	assert.Equal(t, "default_0_5_0", prov.URN.Name())
	assert.Equal(t, resource.NewPropertyMapFromMap(map[string]any{
		"version": "0.5.0",
		"__internal": map[string]any{
			"name":             "pkgA",
			"version":          "1.0.0",
			"parameterization": "cmVwbGFjZW1lbnQ=",
		},
	}), prov.Inputs)

	// Check the state of the imported resources is what we expect
	resA := snap.Resources[1]
	assert.Equal(t, tokens.Type("pkgA:m:typA"), resA.Type)
	assert.Equal(t, "resA", resA.URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("input"),
	}, resA.Inputs)
	assert.Equal(t, resource.PropertyMap{
		"output": resource.NewStringProperty("output"),
	}, resA.Outputs)

	resB := snap.Resources[3]
	assert.Equal(t, tokens.Type("pkgExt:m:typA"), resB.Type)
	assert.Equal(t, "resB", resB.URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("input"),
	}, resB.Inputs)
	assert.Equal(t, resource.PropertyMap{
		"output": resource.NewStringProperty("output"),
	}, resB.Outputs)

	resC := snap.Resources[5]
	assert.Equal(t, tokens.Type("pkgExt:m:typA"), resC.Type)
	assert.Equal(t, "resC", resC.URN.Name())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("input"),
	}, resC.Inputs)
	assert.Equal(t, resource.PropertyMap{
		"output": resource.NewStringProperty("output"),
	}, resC.Outputs)
}
