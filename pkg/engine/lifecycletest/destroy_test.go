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

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Test that we can run a simple destroy by executing the program for it.
func TestDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA and resB
	assert.Equal(t, int32(2), deleteCalled)
	// Resources should be deleted from state
	assert.Len(t, snap.Resources, 0)
}

// Test that we can run a targeted destroy by executing the program for it.
func TestTargetedDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
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
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "resB", snap.Resources[1].URN.Name())
}

// Test that we can run a destroy by executing the program for it, and that in that update we change provider version.
func TestProviderUpdateDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
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
	assert.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it, and that in that update we change provider version.
// This is the same as TestProviderUpdateDestroyWithProgram but with explicit provider references.
func TestExplicitProviderUpdateDestroyWithProgram(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
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
	assert.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program creates components.
func TestDestroyWithProgramWithComponents(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	assert.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program creates components which
// need to be skipped due to another skipped custom resource.
func TestDestroyWithProgramWithSkippedComponents(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	assert.Len(t, snap.Resources, 0)
}

// Test that we can run a destroy by executing the program for it when that program now aliases _and_ skips
// the resource to destroy.
func TestDestroyWithProgramWithSkippedAlias(t *testing.T) {
	t.Parallel()

	programInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createInputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
	createOutputs := resource.PropertyMap{"foo": resource.NewStringProperty("baz")}

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
	programInputs["foo"] = resource.NewStringProperty("qux")
	// Run a destroy
	snap, err = lt.TestOp(DestroyV2).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Should have run the program again
	assert.Equal(t, 2, programExecutions)
	// Should have deleted resA
	assert.Equal(t, 1, deleteCalled)
	// Everything should be deleted from state
	assert.Len(t, snap.Resources, 0)
}
