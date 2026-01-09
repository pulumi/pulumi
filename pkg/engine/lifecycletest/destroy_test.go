// Copyright 2025, Pulumi Corporation.
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
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Test that we can run a simple destroy by executing the program for it.
func TestDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	var deleteCalled int32
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" || req.Name == "resB" {
						atomic.AddInt32(&deleteCalled, 1)
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" || req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: programInputs,
		})
		require.NoError(t, err)
		// Should see the create outputs both times we run this program
		assert.Equal(t, createOutputs, resp.Outputs)

		// Only register resB on the first run
		if programExecutions == 1 {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: programInputs,
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA and resB
	assert.Equal(t, int32(2), deleteCalled)
	// Resources should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Test that we can run a targeted destroy by executing the program for it.
func TestTargetedDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	deleteCalled := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" || req.Name == "resB" {
						deleteCalled++
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" || req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: programInputs,
		})
		require.NoError(t, err)
		// Should see the create outputs both times we run this program
		assert.Equal(t, createOutputs, resp.Outputs)

		// Only register resB on the first run
		if programExecutions == 1 {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: programInputs,
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a targeted destroy against resA
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{snap.Resources[1].URN})
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// resA should be deleted from state
	require.Len(t, snap.Resources, 2)
	assert.Equal(t, "resB", snap.Resources[1].URN.Name())
}

// Test that we can run a destroy by executing the program for it, and that in that update we change provider version.
func TestProviderUpdateDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	var deleteCalled int32
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resB" {
						atomic.AddInt32(&deleteCalled, 1)
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on 1.0 for %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" || req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" {
						atomic.AddInt32(&deleteCalled, 1)
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on 2.0 for %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("should not have called create")
				},
			}, nil
		}),
	}

	programExecutions := 0
	pkgVersion := "1.0.0"
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:  programInputs,
			Version: pkgVersion,
		})
		require.NoError(t, err)
		// Should see the create outputs both times we run this program
		assert.Equal(t, createOutputs, resp.Outputs)

		// Only register resB on the first run
		if programExecutions == 1 {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:  programInputs,
				Version: pkgVersion,
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy with the new provider version
	pkgVersion = "2.0.0"
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA and resB
	assert.Equal(t, int32(2), deleteCalled)
	// All the resources should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it, and that in that update we change provider version.
// This is the same as TestProviderUpdateDestroyWithProgram but with explicit provider references.
func TestExplicitProviderUpdateDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	var deleteCalled int32
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" || req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" || req.Name == "resB" {
						atomic.AddInt32(&deleteCalled, 1)
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on 2.0 for %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("should not have called create")
				},
			}, nil
		}),
	}

	programExecutions := 0
	pkgVersion := "1.0.0"
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
			Version: pkgVersion,
		})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:   programInputs,
			Provider: provRef.String(),
		})
		require.NoError(t, err)
		// Should see the create outputs both times we run this program
		assert.Equal(t, createOutputs, resp.Outputs)

		// Only register resB on the first run
		if programExecutions == 1 {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:   programInputs,
				Provider: provRef.String(),
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy with the new provider version
	pkgVersion = "2.0.0"
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA and resB
	assert.Equal(t, int32(2), deleteCalled)
	// All the resources should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program creates components.
func TestDestroyWithProgramWithComponents(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	deleteCalled := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" || req.Name == "resB" {
						deleteCalled++
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" || req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{
						ID:         resource.ID(uuid.String()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		resp, err := monitor.RegisterResource("my_component", "parent", false)
		require.NoError(t, err)

		resp, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: programInputs,
			Parent: resp.URN,
		})
		require.NoError(t, err)
		assert.Equal(t, createOutputs, resp.Outputs)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program creates components which
// need to be skipped due to another skipped custom resource.
func TestDestroyWithProgramWithSkippedComponents(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	deleteCalled := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" {
						deleteCalled++
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{}, fmt.Errorf("should not have called create on %s", req.URN)
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		if programExecutions == 1 {
			// First execution just create a custom resource
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: programInputs,
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		} else {
			// Second execution (the deletion) we create a custom resource, then a component that depends on
			// that custom resource, then our original custom resource is created as a child of that
			// component.

			// Create a custom resource that will be skipped
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
			require.NoError(t, err)

			// Create a component that depends on the custom resource, it also has to be skipped
			resp, err = monitor.RegisterResource("my_component", "parent", false, deploytest.ResourceOptions{
				Dependencies: []resource.URN{resp.URN},
			})
			require.NoError(t, err)

			// And then create the original custom resource as a child of the component, remember to alias it
			resp, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:       programInputs,
				Dependencies: []resource.URN{resp.URN},
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program now aliases _and_ skips
// the resource to destroy.
func TestDestroyWithProgramWithSkippedAlias(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	deleteCalled := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resA" {
						deleteCalled++
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resA" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{}, fmt.Errorf("should not have called create on %s", req.URN)
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		if programExecutions == 1 {
			// First execution just create a custom resource
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: programInputs,
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		} else {
			// Second execution (the deletion) we create a custom resource that will have to be skipped, then
			// re-parent our existing resource to it with an alias.

			// Create a custom resource that will be skipped
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
			require.NoError(t, err)

			// And then create the original custom resource as a child of the component, remember to alias it
			resp, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: programInputs,
				Parent: resp.URN,
				Aliases: []*pulumirpc.Alias{
					{
						Alias: &pulumirpc.Alias_Spec_{
							Spec: &pulumirpc.Alias_Spec{
								Parent: &pulumirpc.Alias_Spec_NoParent{
									NoParent: true,
								},
							},
						},
					},
				},
			})
			require.NoError(t, err)
			assert.Equal(t, createOutputs, resp.Outputs)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, programInputs, snap.Resources[1].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[1].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	require.Len(t, snap.Resources, 0)
}

// Regression test for https://github.com/pulumi/pulumi/issues/19363. Check that a read resource (i.e.
// Resource.get) doesn't remain in the state after a destroy --run-program operation.
func TestDestroyWithProgramResourceRead(t *testing.T) {
	t.Parallel()

	readInputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}
	readOutputs := resource.PropertyMap{"foo": resource.NewProperty("bar")}

	programInputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}
	createInputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}
	createOutputs := resource.PropertyMap{"foo": resource.NewProperty("baz")}

	deleteCalled := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.Name == "resB" {
						deleteCalled++
						assert.Equal(t, createInputs, req.Inputs)
						assert.Equal(t, createOutputs, req.Outputs)

						return plugin.DeleteResponse{
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.DeleteResponse{}, fmt.Errorf("should not have called delete on %s", req.URN)
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					uuid, err := uuid.NewV4()
					if err != nil {
						return plugin.CreateResponse{}, err
					}

					if req.Name == "resB" {
						assert.Equal(t, programInputs, req.Properties)

						return plugin.CreateResponse{
							ID:         resource.ID(uuid.String()),
							Properties: createOutputs,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{}, fmt.Errorf("should not have called create on %s", req.URN)
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.Name == "resA" {
						assert.Equal(t, resource.ID("id"), req.ID)
						assert.Empty(t, req.Inputs)
						assert.Empty(t, req.State)

						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  readInputs,
								Outputs: readOutputs,
								ID:      req.ID,
							},
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.ReadResponse{}, fmt.Errorf("should not have called read on %s", req.URN)
				},
			}, nil
		}),
	}

	programExecutions := 0
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		programExecutions++

		_, state, err := monitor.ReadResource(
			"pkgA:m:typA",
			"resA",
			"id",
			"",
			resource.PropertyMap{},
			"",
			"",
			"",
			nil,
			"",
			"",
		)
		require.NoError(t, err)
		assert.Equal(t, readOutputs, state)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: programInputs,
		})
		require.NoError(t, err)
		assert.Equal(t, createOutputs, resp.Outputs)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
		},
	}

	// Run an update to create the initial state.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, 1, programExecutions)
	assert.Equal(t, resource.PropertyMap{}, snap.Resources[1].Inputs)
	assert.Equal(t, readOutputs, snap.Resources[1].Outputs)
	assert.Equal(t, programInputs, snap.Resources[2].Inputs)
	assert.Equal(t, createOutputs, snap.Resources[2].Outputs)

	// Change the program inputs to check we don't send changed inputs to the provider
	programInputs["foo"] = resource.NewProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	require.Len(t, snap.Resources, 0)
}

func TestTargetedAliasDestroyV2(t *testing.T) {
	t.Parallel()

	// TODO[https://github.com/pulumi/pulumi/issues/21281]: Fix snapshot integrity issue
	t.Skip("Skipping due to snapshot integrity issue")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::provA",
			Custom: true,
			ID:     "id-prov",
		}
		s.Resources = append(s.Resources, prov)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		comp := &resource.State{
			Type:     "pkgA:m:typA",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA::compA",
			Custom:   false,
			Delete:   true,
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, comp)

		res := &resource.State{
			Type:     "pkgA:m:typB",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA$pkgA:m:typB::resA",
			Custom:   true,
			ID:       "id-res",
			Provider: provRef.String(),
			Parent:   comp.URN,
		}
		s.Resources = append(s.Resources, res)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			AliasURNs: []resource.URN{
				"urn:pulumi:test-stack::test-project::pkgA:m:typA$pkgA:m:typB::resA",
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: engine.UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test-stack::test-project::pkgA:m:typA::compA",
				"urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::provA",
			}),
		},
	}

	_, err := lt.TestOp(engine.DestroyV2).
		RunStep(project, p.GetTarget(t, setupSnap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

func TestDestroyV2ProtectedWithProviderDependencies(t *testing.T) {
	t.Parallel()

	initialSnap := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::prov",
				Custom: true,
				ID:     "prov",
			},
			{
				Type:     "pkgA:m:typA",
				URN:      "urn:pulumi:test::test::pkgA:m:typA::resA",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::prov::prov",
				ID:       "0",
				Protect:  true,
			},
		},
	}
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		protect := true

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Protect:  &protect,
			Provider: provRef.String(),
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}
	p := &lt.TestPlan{
		Options: opts,
	}

	_, err := lt.TestOp(DestroyV2).RunStep(
		p.GetProject(), p.GetTarget(t, initialSnap), opts, false, p.BackendClient, nil, "0")
	require.ErrorContains(t, err, "BAIL: step executor errored: step application failed: resource"+
		" \"urn:pulumi:test::test::pkgA:m:typA::resA\" cannot be deleted")
	require.NotContains(t, err.Error(), "validation error")
}

// TestDestroyWithProgramProtectedResourceWithProvider tests that a protected resource that references a provider
// can be deleted by DestroyV2 without causing a snapshot integrity error.
func TestDestroyWithProgramProtectedResourceWithProvider(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21277]: Remove this once the underlying issue is fixed.
	t.Skip("Skipping test, see pulumi/pulumi#21277")

	initialSnap := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::prov",
				Custom: true,
				ID:     "id-123",
			},
			{
				Type:     "pkgA:m:typA",
				URN:      "urn:pulumi:test::test::pkgA:m:typA::resA",
				Custom:   false,
				ID:       "",
				Protect:  true,
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::prov::id-123",
			},
		},
	}

	require.NoError(t, initialSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	_, err := lt.TestOp(engine.DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, initialSnap), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

func TestDestroyV2TargetChildWithNewParent(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21347]: Remove this once the underlying issue is fixed.
	t.Skip("Skipping test, see pulumi/pulumi#21347")

	initialSnap := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::prov",
				Custom: true,
				ID:     "prov",
			},
			{
				Type:     "pkgA:m:typA",
				URN:      "urn:pulumi:test::test::pkgA:m:typA::future-parent",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::prov::prov",
			},
			{
				Type:   "pulumi:providers:pkgB",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgB::future-child",
				Custom: true,
				ID:     "prov",
			},
		},
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov0, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		prov0Ref, err := providers.NewReference(prov0.URN, prov0.ID)
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:m:typA", "future-parent", false, deploytest.ResourceOptions{
			RetainOnDelete: ptr(true),
			Provider:       prov0Ref.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkgB", "future-child", true, deploytest.ResourceOptions{
			Parent: res1.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::future-parent",
			}),
		},
	}

	p := &lt.TestPlan{
		Options: opts,
	}

	_, err := lt.TestOp(DestroyV2).RunStep(
		p.GetProject(), p.GetTarget(t, initialSnap), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// TestDestroyV2TargetProviderWithAliasedParent tests a targeted destroy of a provider
// whose parent has been aliased to change its parent relationship, which previously
// caused a snapshot integrity error (parent not found in urnIndex).
func TestDestroyV2TargetProviderWithAliasedParent(t *testing.T) {
	t.Parallel()
	// TODO[pulumi/pulumi#21364]: Remove this once the underlying issue is fixed.
	t.Skip("Skipping test, snapshot integrity error with aliased parent")

	initialSnap := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::prov",
				Custom: true,
				ID:     "id-123",
			},
			{
				Type:     "pkgA:mod:ComponentParent",
				URN:      "urn:pulumi:test::test::pkgA:mod:ComponentParent::parent",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::prov::id-123",
			},
			{
				Type:     "pkgA:mod:ComponentChild",
				URN:      "urn:pulumi:test::test::pkgA:mod:ComponentParent$pkgA:mod:ComponentChild::child",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::prov::id-123",
				Parent:   "urn:pulumi:test::test::pkgA:mod:ComponentParent::parent",
			},
			{
				Type:   "pulumi:providers:pkgB",
				URN:    "urn:pulumi:test::test::pkgA:mod:ComponentParent$pkgA:mod:ComponentChild$pulumi:providers:pkgB::childprov",
				Custom: true,
				ID:     "id-456",
				Parent: "urn:pulumi:test::test::pkgA:mod:ComponentParent$pkgA:mod:ComponentChild::child",
			},
		},
	}

	require.NoError(t, initialSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Program aliases the child component to remove its parent,  but the provider still has the child as
	// its parent with the old URN.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		child, err := monitor.RegisterResource("pkgA:mod:ComponentChild", "child", false, deploytest.ResourceOptions{
			Provider: provRef.String(),
			AliasURNs: []resource.URN{
				"urn:pulumi:test::test::pkgA:mod:ComponentParent$pkgA:mod:ComponentChild::child",
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkgB", "childprov", true, deploytest.ResourceOptions{
			Parent: child.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
			UpdateOptions: engine.UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"urn:pulumi:test::test::pkgA:mod:ComponentParent$pkgA:mod:ComponentChild$pulumi:providers:pkgB::childprov",
				}),
			},
		},
	}

	_, err := lt.TestOp(DestroyV2).RunStep(
		p.GetProject(), p.GetTarget(t, initialSnap), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

func TestDestroyV2ResourceWithDependencyOnDeleted(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#21384]: Remove this once the underlying issue is fixed.
	t.Skip("Skipping test, repro for snapshot integrity issue")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov := &resource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::prov",
			Custom: true,
			ID:     "id1",
		}
		s.Resources = append(s.Resources, prov)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		resA1 := &resource.State{
			Type:     "pkgA:m:typA",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA::resA",
			Custom:   false,
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, resA1)

		resA2 := &resource.State{
			Type:     "pkgA:m:typA",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA::resA",
			Custom:   false,
			Delete:   true,
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, resA2)

		resB := &resource.State{
			Type:               "pkgA:m:typB",
			URN:                "urn:pulumi:test-stack::test-project::pkgA:m:typB::resB",
			Custom:             true,
			Delete:             true,
			ID:                 "id2",
			PendingReplacement: true,
			Provider:           provRef.String(),
			Dependencies:       []resource.URN{resA2.URN},
		}
		s.Resources = append(s.Resources, resB)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: engine.UpdateOptions{
			Refresh: true,
		},
	}

	_, err := lt.TestOp(engine.DestroyV2).
		RunStep(project, p.GetTarget(t, setupSnap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
