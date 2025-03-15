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
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestMigrateType checks that the engine calls migrate on resources when their type changes due to an alias.
func TestMigrateType(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Create a resource with a property that will be migrated.
					properties := resource.NewPropertyMapFromMap(map[string]interface{}{
						"preout": req.Properties["prein"].StringValue() + " world",
					})

					return plugin.CreateResponse{
						ID:         "preid",
						Properties: properties,
						Status:     resource.StatusOK,
					}, nil
				},
				MigrateF: func(_ context.Context, req plugin.MigrateRequest) (plugin.MigrateResponse, error) {
					assert.Equal(t, resource.ID("preid"), req.ID)
					// This is nil because we didn't send an explicit type for the provider when registering
					// it.
					assert.Nil(t, req.OldVersion)

					// Rename preX to postX, and update the ID
					newInputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postin": req.OldInputs["prein"],
					})
					newOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postout": req.OldOutputs["preout"],
					})
					return plugin.MigrateResponse{
						NewID:      "postid",
						NewInputs:  newInputs,
						NewOutputs: newOutputs,
					}, nil
				},
			}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"prein": resource.NewStringProperty("hello"),
			},
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("preid"), resp.ID)

		return err
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Assert the resources are the pre-state
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].URN.Type())
	assert.Equal(t, resource.ID("preid"), snap.Resources[1].ID)
	assert.Equal(t, resource.PropertyMap{"prein": resource.NewStringProperty("hello")}, snap.Resources[1].Inputs)
	assert.Equal(t, resource.PropertyMap{"preout": resource.NewStringProperty("hello world")}, snap.Resources[1].Outputs)

	// New run the program again with an alias to change type
	program = func(monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"postin": resource.NewStringProperty("hello"),
			},
			Aliases: []*pulumirpc.Alias{
				{Alias: &pulumirpc.Alias_Spec_{Spec: &pulumirpc.Alias_Spec{Type: "pkgA:m:typA"}}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("postid"), resp.ID)

		return err
	}

	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Assert the resources are the post-state
	assert.Equal(t, tokens.Type("pkgA:m:typB"), snap.Resources[1].URN.Type())
	assert.Equal(t, resource.ID("postid"), snap.Resources[1].ID)
	assert.Equal(t, resource.PropertyMap{"postin": resource.NewStringProperty("hello")}, snap.Resources[1].Inputs)
	assert.Equal(t, resource.PropertyMap{"postout": resource.NewStringProperty("hello world")}, snap.Resources[1].Outputs)
}

// TestMigrateVersion checks that the engine calls migrate on resources when their version changes.
func TestMigrateVersion(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Create a resource with a property that will be migrated.
					properties := resource.NewPropertyMapFromMap(map[string]interface{}{
						"preout": req.Properties["prein"].StringValue() + " world",
					})

					return plugin.CreateResponse{
						ID:         "preid",
						Properties: properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				MigrateF: func(_ context.Context, req plugin.MigrateRequest) (plugin.MigrateResponse, error) {
					assert.Equal(t, resource.ID("preid"), req.ID)
					version := semver.MustParse("1.0.0")
					assert.Equal(t, &version, req.OldVersion)

					// Rename preX to postX, and update the ID
					newInputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postin": req.OldInputs["prein"],
					})
					newOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postout": req.OldOutputs["preout"],
					})
					return plugin.MigrateResponse{
						NewID:      "postid",
						NewInputs:  newInputs,
						NewOutputs: newOutputs,
					}, nil
				},
			}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"prein": resource.NewStringProperty("hello"),
			},
			Version: "1.0.0",
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("preid"), resp.ID)

		return err
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Assert the resources are the pre-state
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].URN.Type())
	assert.Equal(t, resource.ID("preid"), snap.Resources[1].ID)
	assert.Equal(t, resource.PropertyMap{"prein": resource.NewStringProperty("hello")}, snap.Resources[1].Inputs)
	assert.Equal(t, resource.PropertyMap{"preout": resource.NewStringProperty("hello world")}, snap.Resources[1].Outputs)

	// New run the program with a new provider version
	program = func(monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"postin": resource.NewStringProperty("hello"),
			},
			Version: "2.0.0",
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("postid"), resp.ID)

		return err
	}

	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Assert the resources are the post-state
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].URN.Type())
	assert.Equal(t, resource.ID("postid"), snap.Resources[1].ID)
	assert.Equal(t, resource.PropertyMap{"postin": resource.NewStringProperty("hello")}, snap.Resources[1].Inputs)
	assert.Equal(t, resource.PropertyMap{"postout": resource.NewStringProperty("hello world")}, snap.Resources[1].Outputs)
}

// TestMigrateDelete checks that the engine calls migrate on resources when their version changes due to
// a delete from state operation.
func TestMigrateDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// Create a resource with a property that will be migrated.
					properties := resource.NewPropertyMapFromMap(map[string]interface{}{
						"preout": req.Properties["prein"].StringValue() + " world",
					})

					return plugin.CreateResponse{
						ID:         "preid",
						Properties: properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					assert.Fail(t, "Delete should not be called")
					return plugin.DeleteResponse{Status: resource.StatusUnknown}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				MigrateF: func(_ context.Context, req plugin.MigrateRequest) (plugin.MigrateResponse, error) {
					assert.Equal(t, resource.ID("preid"), req.ID)
					version := semver.MustParse("1.0.0")
					assert.Equal(t, &version, req.OldVersion)

					// Rename preX to postX, and update the ID
					newInputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postin": req.OldInputs["prein"],
					})
					newOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"postout": req.OldOutputs["preout"],
					})
					return plugin.MigrateResponse{
						NewID:      "postid",
						NewInputs:  newInputs,
						NewOutputs: newOutputs,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					assert.Equal(t, resource.ID("postid"), req.ID)
					assert.Equal(t, resource.PropertyMap{
						"postin": resource.NewStringProperty("hello"),
					}, req.Inputs)
					assert.Equal(t, resource.PropertyMap{
						"postout": resource.NewStringProperty("hello world"),
					}, req.Outputs)

					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "pkgA", true, deploytest.ResourceOptions{
			Version: "1.0.0",
		})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"prein": resource.NewStringProperty("hello"),
			},
			Provider: provRef.String(),
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("preid"), resp.ID)

		// Create another resource that will only be created in the first run and so be deleted in the second destroy run.
		resp, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"prein": resource.NewStringProperty("hello"),
			},
			Provider: provRef.String(),
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("preid"), resp.ID)

		return err
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	// Assert the resources are the pre-state
	for i := 1; i < len(snap.Resources); i++ {
		assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[i].URN.Type())
		assert.Equal(t, resource.ID("preid"), snap.Resources[i].ID)
		assert.Equal(t, resource.PropertyMap{"prein": resource.NewStringProperty("hello")}, snap.Resources[i].Inputs)
		assert.Equal(t, resource.PropertyMap{"preout": resource.NewStringProperty("hello world")}, snap.Resources[i].Outputs)
	}

	// New run the program with a new provider version and without resB
	program = func(monitor *deploytest.ResourceMonitor) error {
		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "pkgA", true, deploytest.ResourceOptions{
			Version: "2.0.0",
		})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"postin": resource.NewStringProperty("hello"),
			},
			Provider: provRef.String(),
		})
		require.NoError(t, err)
		assert.Equal(t, resource.ID("postid"), resp.ID)

		return err
	}

	snap, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Assert the resources are the post-state
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].URN.Type())
	assert.Equal(t, resource.ID("postid"), snap.Resources[1].ID)
	assert.Equal(t, resource.PropertyMap{"postin": resource.NewStringProperty("hello")}, snap.Resources[1].Inputs)
	assert.Equal(t, resource.PropertyMap{"postout": resource.NewStringProperty("hello world")}, snap.Resources[1].Outputs)
}
