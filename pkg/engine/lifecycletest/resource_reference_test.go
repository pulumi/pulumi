// Copyright 2020-2024, Pulumi Corporation.
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

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
						assert.True(t, req.Properties.DeepEquals(resource.PropertyMap{
							"resA": resource.MakeComponentResourceReference(urnA, ""),
							"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
						}))
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
							Inputs:  req.Inputs,
							Outputs: req.State,
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
		assert.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)
		urnB, idB = respB.URN, respB.ID

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"resA": resource.MakeComponentResourceReference(urnA, ""),
				"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
			},
		})
		assert.NoError(t, err)

		assert.True(t, resp.Outputs.DeepEquals(resource.PropertyMap{
			"resA": resource.MakeComponentResourceReference(urnA, ""),
			"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
		}))
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

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

					state := resource.PropertyMap{}
					if req.URN.Name() == "resC" {
						state = resource.PropertyMap{
							"resA": resource.MakeComponentResourceReference(urnA, ""),
							"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
						}
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
							Inputs:  req.Inputs,
							Outputs: req.State,
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
		assert.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, opts)
		assert.NoError(t, err)
		urnB, idB = respB.URN, respB.ID

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, opts)
		assert.NoError(t, err)

		assert.Equal(t, resource.NewStringProperty(string(urnA)), respC.Outputs["resA"])
		if idB != "" {
			assert.Equal(t, resource.NewStringProperty(string(idB)), respC.Outputs["resB"])
		} else {
			assert.True(t, respC.Outputs["resB"].IsComputed())
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

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
						assert.Equal(t, resource.NewStringProperty(string(urnA)), req.Properties["resA"])
						assert.Equal(t, refB.ResourceReferenceValue().ID, req.Properties["resB"])
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
							Inputs:  req.Inputs,
							Outputs: req.State,
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
		assert.NoError(t, err)
		urnA = respA.URN

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		refB = resource.MakeCustomResourceReference(respB.URN, respB.ID, "")
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"resA": resource.MakeComponentResourceReference(urnA, ""),
				"resB": refB,
			},
		})
		assert.NoError(t, err)

		assert.Equal(t, resource.NewStringProperty(string(urnA)), resp.Outputs["resA"])
		if refB.ResourceReferenceValue().ID.IsComputed() {
			assert.True(t, resp.Outputs["resB"].IsComputed())
		} else {
			assert.True(t, refB.ResourceReferenceValue().ID.DeepEquals(resp.Outputs["resB"]))
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

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
							Inputs:  req.Inputs,
							Outputs: req.State,
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
		assert.NoError(t, err)

		refChild := resource.MakeCustomResourceReference(childResp.URN, childResp.ID, "")
		resp, err := monitor.RegisterResource("pkgA:m:typContainer", "resContainer", true,
			deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"child": refChild,
				},
			})
		assert.NoError(t, err)

		// Expect the `child` property from `resContainer`'s state to come back from 'pulumi:pulumi:getResource'
		// as a resource reference.
		result, failures, err := monitor.Invoke("pulumi:pulumi:getResource", resource.PropertyMap{
			"urn": resource.NewStringProperty(string(resp.URN)),
		}, "", "", "")
		assert.NoError(t, err)
		assert.Empty(t, failures)
		assert.Equal(t, resource.NewStringProperty(string(resp.URN)), result["urn"])
		assert.Equal(t, resource.NewStringProperty(string(resp.ID)), result["id"])
		state := result["state"].ObjectValue()
		assert.Equal(t, refChild, state["child"])

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}
