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
	"strconv"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/sdk/v3/pkg/engine"	//nolint:revive
	lt "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// TestTaintReplacement tests that a tainted resource is replaced on update.
func TestTaintReplacement(t *testing.T) {
	t.Parallel()

	var id int
	var deleteCalls int
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id++
					return plugin.CreateResponse{
						ID:		resource.ID(strconv.Itoa(id)),
						Properties:	req.Properties,
						Status:		resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					assert.Equal(t, resource.ID("1"), req.ID, "should be deleting the original resource")
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register the resource
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty("bar"),
			},
		})
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// Run initial update to create the resource
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	require.Len(t, snap.Resources, 2)	// stack + resA

	// Find resA and taint it
	var resA *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "resA" {
			resA = r
			break
		}
	}
	require.NotNil(t, resA)
	assert.Equal(t, resource.ID("1"), resA.ID)
	resA.Taint = true

	// Run update with the tainted resource
	snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	require.Len(t, snap.Resources, 2)	// stack + replaced resA

	// Verify the resource was replaced
	var newResA *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "resA" {
			newResA = r
			break
		}
	}
	require.NotNil(t, newResA)
	assert.Equal(t, resource.ID("2"), newResA.ID)
	assert.False(t, newResA.Taint, "taint should be cleared after replacement")
	assert.Equal(t, 1, deleteCalls, "delete should have been called for the tainted resource")
}

// TestTaintMultipleResources tests that multiple tainted resources are all replaced.
func TestTaintMultipleResources(t *testing.T) {
	t.Parallel()

	createIDs := make(map[string]int)
	var deleteCalls int
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					name := req.URN.Name()
					createIDs[name]++
					return plugin.CreateResponse{
						ID:		resource.ID(name + "-v" + string(rune('0'+createIDs[name]))),
						Properties:	req.Properties,
						Status:		resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					assert.Contains(t, []resource.ID{"resA-v1", "resC-v1"}, req.ID,
						"should be deleting a tainted resource")
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register three resources
		for _, name := range []string{"resA", "resB", "resC"} {
			_, err := monitor.RegisterResource("pkgA:m:typA", name, true, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"name": resource.NewProperty(name),
				},
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// Run initial update to create resources
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	require.Len(t, snap.Resources, 4)	// stack + 3 resources

	// Taint resA and resC, but not resB
	for _, r := range snap.Resources {
		if r.URN.Name() == "resA" || r.URN.Name() == "resC" {
			r.Taint = true
		}
	}

	// Run update with tainted resources
	snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	require.Len(t, snap.Resources, 4)	// stack + 3 resources

	// Check that taint is cleared and IDs are updated for replaced resources
	replacedCount := 0
	for _, r := range snap.Resources {
		assert.False(t, r.Taint, "all taints should be cleared")
		switch r.URN.Name() {
		case "resA":
			assert.Equal(t, "resA-v2", string(r.ID), "resA should have new ID from replacement")
			replacedCount++
		case "resB":
			assert.Equal(t, "resB-v1", string(r.ID), "resB should have original ID")
		case "resC":
			assert.Equal(t, "resC-v2", string(r.ID), "resC should have new ID from replacement")
			replacedCount++
		}
	}
	assert.Equal(t, 2, replacedCount, "should have replaced 2 resources")
	assert.Equal(t, 2, deleteCalls, "should have deleted 2 resources")
}

// TestTaintWithPendingDelete tests that resources marked for deletion are not affected by taint.
func TestTaintWithPendingDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:		"new-id",
						Properties:	req.Properties,
						Status:		resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register the current resource
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty("bar"),
			},
		})
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create a snapshot with two copies of a resource:
	// - One that is current and tainted
	// - One that is pending deletion and also tainted (should be ignored)
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:	resURN.Type(),
				URN:	resURN,
				Custom:	true,
				ID:	"current-id",
				Inputs: resource.PropertyMap{
					"foo": resource.NewProperty("bar"),
				},
				Outputs: resource.PropertyMap{
					"foo": resource.NewProperty("bar"),
				},
				Taint:	true,	// This resource is tainted and should be replaced
			},
			{
				Type:	resURN.Type(),
				URN:	resURN,
				Custom:	true,
				ID:	"old-id",
				Inputs: resource.PropertyMap{
					"foo": resource.NewProperty("old"),
				},
				Outputs: resource.PropertyMap{
					"foo": resource.NewProperty("old"),
				},
				Delete:	true,	// This resource is marked for deletion
				Taint:	true,	// Taint on deleted resource should be ignored
			},
		},
	}

	// Run update
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, old), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	// Verify results:
	// - The deleted resource should be gone
	// - The current resource should be replaced (due to taint)
	// - There should be only one resource with this URN in the final snapshot
	var finalResource *resource.State
	resourceCount := 0
	for _, r := range snap.Resources {
		if r.URN == resURN {
			finalResource = r
			resourceCount++
		}
	}
	assert.Equal(t, 1, resourceCount, "should have exactly one resource with this URN")
	require.NotNil(t, finalResource)
	assert.Equal(t, resource.ID("new-id"), finalResource.ID, "resource should be replaced")
	assert.False(t, finalResource.Taint, "taint should be cleared after replacement")
	assert.False(t, finalResource.Delete, "resource should not be marked for deletion")
}

// TestTaintNoChanges tests that a tainted resource forces replacement even when there are no other changes.
func TestTaintNoChanges(t *testing.T) {
	t.Parallel()

	var id int
	var deleteCalls int
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id++
					return plugin.CreateResponse{
						ID:		resource.ID("id-" + string(rune('0'+id))),
						Properties:	req.Properties,
						Status:		resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					// No changes in properties
					return plugin.DiffResponse{
						Changes: plugin.DiffNone,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register the same resource with same properties
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty("bar"),
			},
		})
		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	// Run initial update
	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	// Run update without taint - should be no-op
	snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, deleteCalls, "no replacement should occur without taint")

	// Find resA and taint it
	var resA *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "resA" {
			resA = r
			break
		}
	}
	require.NotNil(t, resA)
	assert.Equal(t, resource.ID("id-1"), resA.ID)
	resA.Taint = true

	// Run update with taint - should force replacement
	snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, deleteCalls, "replacement should occur due to taint")

	// Verify taint is cleared
	var newResA *resource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "resA" {
			newResA = r
			break
		}
	}
	require.NotNil(t, newResA)
	assert.False(t, newResA.Taint, "taint should be cleared after replacement")
	assert.Equal(t, resource.ID("id-2"), newResA.ID, "resource should have new ID")
}
