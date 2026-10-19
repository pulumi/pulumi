// Copyright 2020, Pulumi Corporation.
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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// TestResourceReferences tests that resource references can be marshaled between the engine, language host,
// resource providers, and statefile if each entity supports resource references.
func TestResourceReferences(t *testing.T) {
	t.Parallel()

	var urnA resource.URN
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					if req.URN.Name() == "resC" {
						idBVal := property.New(string(idB))
						if idB == "" {
							idBVal = property.New(property.Computed)
						}
						assert.True(t, req.Properties.Equals(property.NewMap(map[string]property.Value{
							"resA": property.New(property.NewResourceReference(urnA, property.Value{}, "")),
							"resB": property.New(property.NewResourceReference(urnB, idBVal, "")),
						})))
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  &req.Inputs,
							Outputs: &req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		respA, err := monitor.RegisterResource("component", "resA", false)
		require.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, property.Map{})
		require.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		require.NoError(t, err)
		urnB, idB = respB.URN, respB.ID

		idBVal := property.New(string(idB))
		if idB == "" {
			idBVal = property.New(property.Computed)
		}
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: property.NewMap(map[string]property.Value{
				"resA": property.New(property.NewResourceReference(urnA, property.Value{}, "")),
				"resB": property.New(property.NewResourceReference(urnB, idBVal, "")),
			}),
		})
		require.NoError(t, err)

		assert.True(t, resp.Outputs.Equals(property.NewMap(map[string]property.Value{
			"resA": property.New(property.NewResourceReference(urnA, property.Value{}, "")),
			"resB": property.New(property.NewResourceReference(urnB, idBVal, "")),
		})))
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_DownlevelSDK tests that resource references are properly marshaled as URNs (for references to
// component resources) or IDs (for references to custom resources) if the SDK does not support resource references.
func TestResourceReferences_DownlevelSDK(t *testing.T) {
	t.Parallel()

	var urnA resource.URN
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					state := property.Map{}
					if req.URN.Name() == "resC" {
						idBVal := property.New(string(idB))
						if idB == "" {
							idBVal = property.New(property.Computed)
						}
						state = property.NewMap(map[string]property.Value{
							"resA": property.New(property.NewResourceReference(urnA, property.Value{}, "")),
							"resB": property.New(property.NewResourceReference(urnB, idBVal, "")),
						})
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: state,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  &req.Inputs,
							Outputs: &req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}),
	}

	opts := deploytest.ResourceOptions{DisableResourceReferences: true}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		respA, err := monitor.RegisterResource("component", "resA", false, opts)
		require.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, property.Map{})
		require.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, opts)
		require.NoError(t, err)
		urnB, idB = respB.URN, respB.ID

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, opts)
		require.NoError(t, err)

		assert.Equal(t, string(urnA), respC.Outputs.Get("resA").AsString())
		if idB != "" {
			assert.Equal(t, string(idB), respC.Outputs.Get("resB").AsString())
		} else {
			assert.True(t, respC.Outputs.Get("resB").IsComputed())
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_DownlevelEngine tests an SDK that supports resource references communicating with an engine
// that does not.
func TestResourceReferences_DownlevelEngine(t *testing.T) {
	t.Parallel()

	var urnA resource.URN
	var refB resource.PropertyValue

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					// If we have resource references here, the engine has not properly disabled them.
					if req.URN.Name() == "resC" {
						assert.Equal(t, string(urnA), req.Properties.Get("resA").AsString())
						assert.Equal(t, refB.ResourceReferenceValue().ID,
							resource.ToResourcePropertyValue(req.Properties.Get("resB")))
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  &req.Inputs,
							Outputs: &req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		respA, err := monitor.RegisterResource("component", "resA", false)
		require.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, property.Map{})
		require.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		require.NoError(t, err)

		refB = resource.MakeCustomResourceReference(respB.URN, respB.ID, "")
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: property.NewMap(map[string]property.Value{
				"resA": property.New(property.NewResourceReference(urnA, property.Value{}, "")),
				"resB": resource.FromResourcePropertyValue(refB),
			}),
		})
		require.NoError(t, err)

		assert.Equal(t, string(urnA), resp.Outputs.Get("resA").AsString())
		if refB.ResourceReferenceValue().ID.IsComputed() {
			assert.True(t, resp.Outputs.Get("resB").IsComputed())
		} else {
			assert.True(t,
				refB.ResourceReferenceValue().ID.DeepEquals(resource.ToResourcePropertyValue(resp.Outputs.Get("resB"))))
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			UpdateOptions:    UpdateOptions{DisableResourceReferences: true},
			SkipDisplayTests: true,
		},
		Steps: lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_GetResource tests that invoking the built-in 'pulumi:pulumi:getResource' function
// returns resource references for any resource reference in a resource's state.
func TestResourceReferences_GetResource(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}
					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  &req.Inputs,
							Outputs: &req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		childResp, err := monitor.RegisterResource("pkgA:m:typChild", "resChild", true)
		require.NoError(t, err)

		refChild := property.New(property.NewResourceReference(childResp.URN, property.New(string(childResp.ID)), ""))
		resp, err := monitor.RegisterResource("pkgA:m:typContainer", "resContainer", true,
			deploytest.ResourceOptions{
				Inputs: property.NewMap(map[string]property.Value{
					"child": refChild,
				}),
			})
		require.NoError(t, err)

		// Expect the `child` property from `resContainer`'s state to come back from 'pulumi:pulumi:getResource'
		// as a resource reference.
		result, failures, err := monitor.Invoke("pulumi:pulumi:getResource", property.NewMap(map[string]property.Value{
			"urn": property.New(string(resp.URN)),
		}), "", "", "")
		require.NoError(t, err)
		assert.Empty(t, failures)
		assert.Equal(t, string(resp.URN), result.Get("urn").AsString())
		assert.Equal(t, string(resp.ID), result.Get("id").AsString())
		state := result.Get("state").AsMap()
		// On preview the ID is computed; after creation it's the actual ID.
		actualRef := state.Get("child").AsResourceReference()
		assert.Equal(t, childResp.URN, actualRef.URN)
		if string(childResp.ID) == "" {
			assert.Equal(t, property.New(property.Computed), actualRef.ID)
		} else {
			assert.Equal(t, property.New(string(childResp.ID)), actualRef.ID)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_NameAndTypeFilledByEngine tests that Name and Type are filled in by the engine before
// passing resource references to Construct calls and program code, even when the originating side omitted them.
func TestResourceReferences_NameAndTypeFilledByEngine(t *testing.T) {
	t.Parallel()

	var sourceURN resource.URN
	var sourceID resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}
					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					ref := req.Inputs.Get("ref").AsResourceReference()
					assert.Equal(t, sourceURN, ref.URN)
					assert.Equal(t, sourceURN.Name(), ref.Name)
					assert.Equal(t, string(sourceURN.Type()), ref.Type)

					component, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)

					// Echo the reference back while intentionally omitting Name/Type to validate
					// the engine fills them before they reach the program.
					outputs := property.NewMap(map[string]property.Value{
						"echo": property.New(property.ResourceReference{
							URN: ref.URN,
							ID:  ref.ID,
						}),
					})
					err = monitor.RegisterResourceOutputs(component.URN, outputs)
					require.NoError(t, err)

					return plugin.ConstructResponse{
						URN:     component.URN,
						Outputs: outputs,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		sourceResp, err := monitor.RegisterResource("pkgA:m:source", "source", true)
		require.NoError(t, err)
		sourceURN, sourceID = sourceResp.URN, sourceResp.ID

		refID := property.New(string(sourceID))
		if sourceID == "" {
			refID = property.New(property.Computed)
		}
		inputRef := property.ResourceReference{
			URN: sourceURN,
			ID:  refID,
		}
		assert.Empty(t, inputRef.Name)
		assert.Empty(t, inputRef.Type)

		componentResp, err := monitor.RegisterResource("pkgA:m:component", "component", false, deploytest.ResourceOptions{
			Remote: true,
			Inputs: property.NewMap(map[string]property.Value{
				"ref": property.New(inputRef),
			}),
		})
		require.NoError(t, err)

		echo := componentResp.Outputs.Get("echo").AsResourceReference()
		assert.Equal(t, sourceURN, echo.URN)
		assert.Equal(t, sourceURN.Name(), echo.Name)
		assert.Equal(t, string(sourceURN.Type()), echo.Type)
		assert.True(t, echo.ID.Equals(refID))

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	snap, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil),
		p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	var componentState *pkgresource.State
	for _, res := range snap.Resources {
		if res.URN.Name() == "component" && res.Type == "pkgA:m:component" {
			componentState = res
			break
		}
	}
	require.NotNil(t, componentState)

	echo := resource.ToResourcePropertyMap(componentState.Outputs)["echo"]
	assert.True(t, echo.IsResourceReference())
}
