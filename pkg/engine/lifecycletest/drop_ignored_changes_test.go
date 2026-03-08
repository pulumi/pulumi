// Copyright 2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDropIgnoredChanges(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"keep":   resource.NewStringProperty("kept-value"),
				"ignore": resource.NewStringProperty("large-ignored-value"),
			},
			IgnoreChanges:      []string{"ignore"},
			DropIgnoredChanges: true,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
		},
		Steps: []lt.TestStep{{
			Op: engine.Update,
			Validate: func(
				project workspace.Project,
				target deploy.Target,
				entries engine.JournalEntries,
				events []engine.Event,
				err error,
			) error {
				require.NoError(t, err)
				for _, entry := range entries {
					if entry.Step.URN().Name() == "resA" {
						state := entry.Step.New()
						assert.True(t, state.DropIgnoredChanges)
						assert.Equal(t, []string{"ignore"}, state.IgnoreChanges)
					}
				}
				return nil
			},
		}},
	}

	snap := p.Run(t, &deploy.Snapshot{})
	require.NotNil(t, snap)

	// Verify that the resource in the snapshot has the dropIgnoredChanges flag set
	for _, res := range snap.Resources {
		if res.URN.Name() == "resA" {
			assert.True(t, res.DropIgnoredChanges)
			assert.Equal(t, []string{"ignore"}, res.IgnoreChanges)
		}
	}
}
