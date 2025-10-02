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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
		// This resource checks that we behave correctly when all updating fields are hidden.
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

		// This resource checks that diff behaves correctly when both hidden and
		// non-hidden fields are diffed.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
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
			},
		})
		require.NoError(t, err)

		// This resource checks that hidden but non-changed fields don't show as a hidden diff.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"scalar": resource.NewProperty("fixed"),
			},
			HideDiffs: []resource.PropertyPath{
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
		Steps: []lt.TestStep{{
			Op: engine.Update,
			// This test validates our display logic, so we don't need to
			// validate events or state.
			Validate: nil,
		}},
	}

	created := p.Run(t, &deploy.Snapshot{})
	propValue = resource.NewProperty("b")
	p.Run(t, created) // Update
}
