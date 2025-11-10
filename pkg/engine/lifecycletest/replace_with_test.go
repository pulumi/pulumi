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
