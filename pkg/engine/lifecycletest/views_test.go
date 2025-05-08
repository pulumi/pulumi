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
	"fmt"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestViewsBasic(t *testing.T) {
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
							},
						},
					})
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter++
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())

	// Run a second update, should be same.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3) // TODO: investigate failure: the view is missing; should have been samed
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())

	// TODO full lifecycle
	// Check events, including summary event counts
}

// TODO:
// - target
// - exclude
// - protect
// - destroy (old style)
// - destroy (run program)
// - refresh (old style)
// - refresh (run program)
// - refresh update (old style)
// - refresh update (run program)
// - view replacements (delete before create)
// - view replacements (create before delete)
// - view replaced with a real resource
// - real resource replaced with a view
// - view nested parenting
// - update plans
// - import resource with views
// - read resource with views
// - pulumi:pulumi:getResource
