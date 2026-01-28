// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func TestReplaceWith(t *testing.T) {
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

	// Any old value that we can change later to trigger a replace.
	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		// When we replace A, we should also replace B.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			ReplaceWith: []resource.URN{respA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)

	// We create 2 resources, A and B. After the first deploy, we shouldn't have deleted anything.
	require.Len(t, created, 2)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String())

	require.Len(t, deleted, 0)

	// Change the property on A, trigger a replacement of A and B.
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// We should have replaced A and therefore B, which means two new creates and two deletes.
	// The two new creates will have the same URNs as the old ones, but different IDs. The two
	// deletes will be the original two objects.
	require.Len(t, created, 4)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	require.Len(t, deleted, 2)
	require.Contains(t, deleted, created[0])
	require.Contains(t, deleted, created[1])
}

func TestReplaceWithAndDeletedWith(t *testing.T) {
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

	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	numberOfRuns := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var urnB, urnC resource.URN

		if numberOfRuns == 0 {
			respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]any{}),
			})

			require.NoError(t, err)
			urnB = respB.URN
		}

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})

		require.NoError(t, err)
		urnC = respC.URN

		opts := deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{}),
			ReplaceWith: []resource.URN{urnC},
		}

		if numberOfRuns == 0 {
			opts.DeletedWith = urnB
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		require.NoError(t, err)

		numberOfRuns++
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")

	require.NoError(t, err)
	require.NotNil(t, snap)

	require.Len(t, snap.Resources, 4)
	require.Equal(t, "created-id-1", snap.Resources[1].ID.String()) // B
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String()) // C
	require.Equal(t, "created-id-3", snap.Resources[3].ID.String()) // A (deletes with B, replaces with C)

	require.Equal(t, snap.Resources[3].DeletedWith, snap.Resources[1].URN)
	require.Contains(t, snap.Resources[3].ReplaceWith, snap.Resources[2].URN)

	inputs["foo"] = resource.NewProperty("baz") // Trigger replacement of C
	// B doesn't exist for the second update, so deletedWith should be triggered.

	created = []resource.URN{}
	deleted = []resource.URN{}

	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.NoError(t, err)
	require.NotNil(t, snap)

	require.Len(t, snap.Resources, 3)
	require.Equal(t, "created-id-1", snap.Resources[1].ID.String()) // C
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String()) // A

	require.Len(t, created, 2) // Created A and C
	require.Len(t, deleted, 2) // Deleted B, replaced C

	require.Contains(t, deleted[0].Name(), "resB")
	require.Contains(t, deleted[1].Name(), "resC")
}

func TestReplaceWithDeleteBeforeReplace(t *testing.T) {
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
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		// When we replace A, we should also replace B.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			ReplaceWith: []resource.URN{respA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)

	// We create 2 resources, A and B. After the first deploy, we shouldn't have deleted anything.
	require.Len(t, created, 2)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String())

	require.Len(t, deleted, 0)

	// Change the property on A, trigger a replacement of A and B.
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// We should have replaced A and therefore B, which means two new creates and two deletes.
	// The two new creates will have the same URNs as the old ones, but different IDs. The two
	// deletes will be the original two objects.
	require.Len(t, created, 4)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	require.Len(t, deleted, 2)
	require.Contains(t, deleted, created[0])
	require.Contains(t, deleted, created[1])
}
