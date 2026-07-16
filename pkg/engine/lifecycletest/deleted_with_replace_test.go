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
	"fmt"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

// A dependent whose DeletedWith names a resource being replaced must itself replace, even
// though its own inputs never change.
func TestDeletedWithDependentReplacedOnDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:             plugin.DiffSome,
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	// Any old value that we can change later to trigger a replace.
	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		// resD's own inputs never change, but it must replace whenever resT does.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			DeletedWith: respT.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Len(t, created, 2)
	require.Len(t, deleted, 0)

	urnT := snap.Resources[1].URN
	urnD := snap.Resources[2].URN

	// Change the property on T, which should force D to replace too despite having no diff of its own.
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Equal(t, urnT, snap.Resources[1].URN)
	require.Equal(t, urnD, snap.Resources[2].URN)

	require.Len(t, created, 4)
	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	// Delete-before-replace deletes the dependent by explicit RPC before T; the DeletedWith
	// delete-skip does not apply because T is tracked as a replace, not a delete.
	require.Equal(t, []resource.URN{urnD, urnT}, deleted)
}

// A DeletedWith chain (T <- D <- D2) must replace transitively when the root is replaced.
func TestDeletedWithTransitiveChain(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:             plugin.DiffSome,
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		respD, err := monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			DeletedWith: respT.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD2", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed2": "property"}),
			DeletedWith: respD.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 4)
	require.Len(t, created, 3)
	require.Len(t, deleted, 0)

	urnT := snap.Resources[1].URN
	urnD := snap.Resources[2].URN
	urnD2 := snap.Resources[3].URN

	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 4)
	require.Equal(t, urnT, snap.Resources[1].URN)
	require.Equal(t, urnD, snap.Resources[2].URN)
	require.Equal(t, urnD2, snap.Resources[3].URN)

	require.Len(t, created, 6)
	require.Equal(t, "created-id-4", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-5", snap.Resources[2].ID.String())
	require.Equal(t, "created-id-6", snap.Resources[3].ID.String())

	// D is deleted by explicit RPC before T, but D2's Delete is skipped because its DeletedWith
	// target D is itself being deleted in this deployment.
	require.Equal(t, []resource.URN{urnD, urnT}, deleted)
}

// The same cascade must apply when T defaults to create-before-delete, since the cascade is
// driven by the diff-path DeletedWith check rather than the delete-before-replace eager scan.
func TestDeletedWithDependentReplacedOnCreateBeforeDelete(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			DeletedWith: respT.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Len(t, created, 2)

	urnT := snap.Resources[1].URN
	urnD := snap.Resources[2].URN

	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Equal(t, urnT, snap.Resources[1].URN)
	require.Equal(t, urnD, snap.Resources[2].URN)

	require.Len(t, created, 4)
	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	require.Len(t, deleted, 1)
	require.Contains(t, deleted, urnT)
	require.NotContains(t, deleted, urnD)
}

// DeletedWith added on D in the same step that force-replaces T must still trigger D's
// replacement, exercising the diff-path check rather than the (stale) old-state link.
func TestDeletedWithAddedSameStepAsReplace(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:             plugin.DiffSome,
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})
	linkD := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		opts := deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
		}
		if linkD {
			opts.DeletedWith = respT.URN
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, opts)
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Len(t, created, 2)

	urnT := snap.Resources[1].URN
	urnD := snap.Resources[2].URN

	// Add the DeletedWith link and force-replace T in the same step.
	linkD = true
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	require.Equal(t, urnT, snap.Resources[1].URN)
	require.Equal(t, urnD, snap.Resources[2].URN)

	require.Len(t, created, 4)
	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())
	require.Equal(t, urnT, snap.Resources[2].DeletedWith)
}

// A targeted destroy of T must cascade to delete D, and D's own Delete RPC must be skipped since
// its deletion is a side effect of T's.
func TestDeletedWithTargetedDestroy(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			DeletedWith: respT.URN,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)

	urnProvider := snap.Resources[0].URN
	urnT := snap.Resources[1].URN
	urnD := snap.Resources[2].URN

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:                t,
		HostF:            hostF,
		SkipDisplayTests: true,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargetsFromUrns([]resource.URN{urnT}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	// Only the untargeted default provider survives.
	require.Len(t, snap.Resources, 1)
	require.Equal(t, urnProvider, snap.Resources[0].URN)

	require.Len(t, deleted, 1)
	require.Contains(t, deleted, urnT)
	require.NotContains(t, deleted, urnD)
}

// Replacing T delete-before-replace must fail if the cascade would require replacing a
// protected dependent D.
func TestDeletedWithProtectedDependentBlocksReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:             plugin.DiffSome,
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respT, err := monitor.RegisterResource("pkgA:m:typA", "resT", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		protect := true
		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			DeletedWith: respT.URN,
			Protect:     &protect,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}
	p := &lt.TestPlan{}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 3)

	ins["foo"] = resource.NewProperty("baz")
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	require.Error(t, err)
	require.ErrorContains(t, err, "marked for protection")
}
