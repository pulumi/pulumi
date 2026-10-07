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
	"strconv"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPreviewRefreshAdoptionDoesNotPersist(t *testing.T) {
	t.Parallel()

	for _, refresh := range []bool{false, true} {
		t.Run(strconv.FormatBool(refresh), func(t *testing.T) {
			t.Parallel()
			p := &lt.TestPlan{}
			providerURN := p.NewURN("pulumi:providers:pkgA", "default", "")
			providerRef, err := providers.NewReference(providerURN, "provider-id")
			require.NoError(t, err)
			urn := p.NewURN("pkgA:index:Bucket", "bucket", "")
			stored := resource.PropertyMap{"versioning": resource.NewProperty(false)}
			cloud := resource.PropertyMap{"versioning": resource.NewProperty(true)}
			originalCloud := cloud.Copy()
			snap := &deploy.Snapshot{Resources: []*pkgresource.State{
				{URN: providerURN, Type: providerURN.Type(), Custom: true, ID: "provider-id"},
				{
					URN: urn, Type: urn.Type(), Custom: true, ID: "bucket-id", Provider: providerRef.String(),
					Inputs: stored.Copy(), Outputs: stored.Copy(),
				},
			}}
			before, err := stack.SerializeDeployment(t.Context(), snap, false)
			require.NoError(t, err)
			var reads, writes int
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
							reads++
							values := resource.FromResourcePropertyMap(cloud)
							return plugin.ReadResponse{
								ReadResult: plugin.ReadResult{ID: req.ID, Inputs: &values, Outputs: &values},
								Status:     resource.StatusOK,
							}, nil
						},
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							if !req.Preview {
								writes++
								cloud = resource.ToResourcePropertyMap(req.Properties)
							}
							return plugin.CreateResponse{ID: "new-bucket", Properties: req.Properties}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							if !req.Preview {
								writes++
								cloud = resource.ToResourcePropertyMap(req.NewInputs)
							}
							return plugin.UpdateResponse{Properties: req.NewInputs}, nil
						},
						DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							writes++
							cloud = nil
							return plugin.DeleteResponse{}, nil
						},
					}, nil
				}),
			}
			program := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				assert.True(t, info.DryRun)
				_, err := monitor.RegisterResource(urn.Type(), urn.Name(), true, deploytest.ResourceOptions{
					Inputs: originalCloud.Copy(),
				})
				return err
			})
			journal := &previewRefreshSnapshotManager{TestJournal: engine.NewTestJournal()}
			var changes display.ResourceChanges
			op := lt.TestOp(func(info engine.UpdateInfo, ctx *engine.Context, opts engine.UpdateOptions, dryRun bool) (
				*deploy.Plan, display.ResourceChanges, error,
			) {
				ctx.SnapshotManager = journal
				plan, result, err := engine.Update(info, ctx, opts, dryRun)
				changes = result
				return plan, result, err
			})
			_, err = op.Run(p.GetProject(), p.GetTarget(t, snap), lt.TestUpdateOptions{
				T: t, HostF: deploytest.NewPluginHostF(nil, nil, program, nil, nil, loaders...),
				UpdateOptions: engine.UpdateOptions{Refresh: refresh}, SkipDisplayTests: true,
			}, true, p.BackendClient, func(_ workspace.Project, _ deploy.Target, _ engine.JournalEntries,
				events []engine.Event, err error,
			) error {
				var completedRefreshes int
				for _, event := range events {
					if event.Type == engine.ResourceOutputsEvent {
						metadata := event.Payload().(engine.ResourceOutputsEventPayload).Metadata
						if metadata.URN == urn && metadata.Op == deploy.OpRefresh {
							completedRefreshes++
							assert.Equal(t, cloud, metadata.New.Inputs)
							assert.Equal(t, resource.ID("bucket-id"), metadata.New.ID)
						}
					}
				}
				if refresh {
					assert.Equal(t, 1, completedRefreshes)
				} else {
					assert.Zero(t, completedRefreshes)
				}
				return err
			})
			require.NoError(t, journal.Close())
			require.NoError(t, err)
			if refresh {
				assert.Equal(t, 1, reads)
				assert.Zero(t, changes[deploy.OpUpdate])
			} else {
				assert.Zero(t, reads)
				assert.Equal(t, 1, changes[deploy.OpUpdate])
			}
			assert.Zero(t, changes[deploy.OpCreate])
			assert.Zero(t, changes[deploy.OpDelete])
			assert.Zero(t, changes[deploy.OpReplace])
			assert.Zero(t, writes)
			assert.Equal(t, originalCloud, cloud)
			assert.Empty(t, journal.Entries(), "preview must not write a checkpoint")
			assert.Zero(t, journal.writes, "preview must not write a snapshot")
			after, err := stack.SerializeDeployment(t.Context(), snap, false)
			require.NoError(t, err)
			assert.Equal(t, before, after, "preview must not modify the saved snapshot")
		})
	}
}

type previewRefreshSnapshotManager struct {
	*engine.TestJournal
	writes int
}

func (m *previewRefreshSnapshotManager) Write(*deploy.Snapshot) error {
	m.writes++
	return nil
}

func (m *previewRefreshSnapshotManager) RebuiltBaseState() error {
	m.writes++
	return nil
}
