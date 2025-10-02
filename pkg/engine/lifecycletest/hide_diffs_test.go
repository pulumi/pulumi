// Copyright 2020-2024, Pulumi Corporation.
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
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestHideDiffs_Plain(t *testing.T) {
	t.Parallel()
	testHideDiffs(t, false)
}

func TestHideDiffs_Detailed(t *testing.T) {
	t.Parallel()
	testHideDiffs(t, true)
}

func testHideDiffs(t *testing.T, detailedDiff bool) {
	propValue := resource.NewProperty("a")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if req.OldInputs.DeepEquals(req.NewInputs) {
						return plugin.DiffResult{
							Changes: plugin.DiffNone,
						}, nil
					}

					p := plugin.DiffResult{Changes: plugin.DiffSome}
					if detailedDiff {
						p.DetailedDiff = plugin.NewDetailedDiffFromObjectDiff(req.OldInputs.Diff(req.NewInputs), true)
					}

					check := func(key resource.PropertyKey) {
						if !req.OldInputs[key].DeepEquals(req.NewInputs[key]) {
							p.ChangedKeys = append(p.ChangedKeys, key)
						}
					}

					check("scalar")
					check("array")
					check("map")

					return p, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"scalar": propValue,
				"array": resource.NewProperty([]resource.PropertyValue{
					propValue,
				}),
				"map": resource.NewProperty(resource.PropertyMap{
					"k": propValue,
				}),
			},
			HideDiffs: []resource.PropertyPath{
				{"array"},
				{"map"},
				{"scalar"},
			},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: false,
		},
		Steps: []lt.TestStep{
			{
				Op: engine.Update,
				Validate: func(
					project workspace.Project, target deploy.Target, entries engine.JournalEntries,
					events []engine.Event, err error,
				) error {
					lt.AssertDisplay(t, events, filepath.Join("testdata", "output", t.Name()))
					return nil
				},
			},
		},
	}

	created := p.Run(t, &deploy.Snapshot{})
	propValue = resource.NewProperty("b")
	p.Run(t, created) // Update
}
