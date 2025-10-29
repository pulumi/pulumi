// Copyright 2025-2025, Pulumi Corporation.
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
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// A basic test that an ephemeral resource is created and then deleted as part of the same update.
func TestSingleEphemeralResource(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.PropertyMap{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if _, has := resources[string(req.ID)]; !has {
						return plugin.DeleteResponse{}, fmt.Errorf("unknown resource ID: %s", req.ID)
					}

					delete(resources, string(req.ID))

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					resources[id.String()] = req.Properties

					return plugin.CreateResponse{
						ID:         resource.ID(id.String()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		props := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
		resp, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
			Inputs:    props,
			Ephemeral: true,
		})
		require.NoError(t, err)
		assert.Equal(t, props, resp.Outputs)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Should just have the provider resource left
	require.Len(t, snap.Resources, 1)
}

// Check that when a resource is ephemeral other resources that depend on it don't save that dependency to the state
// file (and cause integrity errors).
func TestSingleEphemeralDependencies(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.PropertyMap{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if _, has := resources[string(req.ID)]; !has {
						return plugin.DeleteResponse{}, fmt.Errorf("unknown resource ID: %s", req.ID)
					}

					delete(resources, string(req.ID))

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					resources[id.String()] = req.Properties

					return plugin.CreateResponse{
						ID:         resource.ID(id.String()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		props := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
		resp, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
			Inputs:    props,
			Ephemeral: true,
		})
		require.NoError(t, err)
		assert.Equal(t, props, resp.Outputs)

		respB, err := monitor.RegisterResource("pkgA:index:typ", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"A": props["A"],
			}),
			Dependencies: []resource.URN{resp.URN},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"A": {resp.URN},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, props, respB.Outputs)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		HostF:            hostF,
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Should have the non-ephemeral resource and the provider resource left
	require.Len(t, snap.Resources, 2)
}

// Test ephemeral resources delete in the expected dependency order.
func TestEphemeralDependencyDeletionOrder(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.PropertyMap{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if _, has := resources[string(req.ID)]; !has {
						return plugin.DeleteResponse{}, fmt.Errorf("unknown resource ID: %s", req.ID)
					}

					// Check if any resource still depends on this resource.
					for id, props := range resources {
						prop := props["A"]
						str := prop.StringValue()
						if str == string(req.ID) {
							return plugin.DeleteResponse{}, fmt.Errorf("resource %s still depended on by resource %s", req.ID, id)
						}
					}

					delete(resources, string(req.ID))

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					resources[id.String()] = req.Properties

					return plugin.CreateResponse{
						ID:         resource.ID(id.String()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		props := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
		resp, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
			Inputs:    props,
			Ephemeral: true,
		})
		require.NoError(t, err)
		assert.Equal(t, props, resp.Outputs)

		respB, err := monitor.RegisterResource("pkgA:index:typ", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"A": resp.ID,
			}),
			Dependencies: []resource.URN{resp.URN},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"A": {resp.URN},
			},
			Ephemeral: true,
		})
		require.NoError(t, err)

		respC, err := monitor.RegisterResource("pkgA:index:typ", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"A": resp.Outputs["A"],
			}),
			Dependencies: []resource.URN{respB.URN},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"A": {respB.URN},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, props, respC.Outputs)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		HostF:            hostF,
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Should have the non-ephemeral resource and the provider resource left
	require.Len(t, snap.Resources, 2)
}

// Test ephemeral resources cause every child resource to also be ephemeral.
func TestEphemeralParenting(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.PropertyMap{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if _, has := resources[string(req.ID)]; !has {
						return plugin.DeleteResponse{}, fmt.Errorf("unknown resource ID: %s", req.ID)
					}

					// Check if any resource still depends on this resource.
					for id, props := range resources {
						prop := props["A"]
						str := prop.StringValue()
						if str == string(req.ID) {
							return plugin.DeleteResponse{}, fmt.Errorf("resource %s still depended on by resource %s", req.ID, id)
						}
					}

					delete(resources, string(req.ID))

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					resources[id.String()] = req.Properties

					return plugin.CreateResponse{
						ID:         resource.ID(id.String()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		props := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
		resp, err := monitor.RegisterResource("pkgA:index:typ", "parent", true, deploytest.ResourceOptions{
			Inputs:    props,
			Ephemeral: true,
		})
		require.NoError(t, err)
		assert.Equal(t, props, resp.Outputs)

		respB, err := monitor.RegisterResource("pkgA:index:typ", "child", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"A": "bar",
			}),
			Parent: resp.URN,
		})
		require.NoError(t, err)

		respC, err := monitor.RegisterResource("pkgA:index:typ", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"A": resp.Outputs["A"],
			}),
			Dependencies: []resource.URN{respB.URN},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"A": {respB.URN},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, props, respC.Outputs)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		HostF:            hostF,
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Should have the non-ephemeral resource and the provider resource left
	require.Len(t, snap.Resources, 2)
}
