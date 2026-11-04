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
	"sync"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readDependsOnProviderLoader builds a provider that records Create/Read calls
// in order. If diffReplace is true, Diff returns a replace result so updating
// src forces a replacement.
func readDependsOnProviderLoader(record func(string), diffReplace bool) []*deploytest.ProviderLoader {
	return []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					if diffReplace {
						return plugin.DiffResponse{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"value"},
						}, nil
					}
					return plugin.DiffResponse{}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					record("create:" + req.URN.Name())
					id := resource.ID("src-id")
					if req.Preview {
						id = ""
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					record("read:" + req.URN.Name())
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Outputs: new(property.Map{}),
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}
}

// TestReadDependsOnOrdering verifies that a ReadResource with dependencies on a
// resource that is being newly created or replaced in the current preview does
// not invoke the provider's Read. In update mode the engine orders Read strictly
// after the dependency's Create.
//
// Covers https://github.com/pulumi/pulumi/issues/16395. The engine treats both
// explicit `dependsOn` and implicit property-reference dependencies the same
// way: SDKs flatten both into the ReadResourceRequest.dependencies field, so a
// single engine code path (step_generator.GenerateReadSteps) covers them.
func TestReadDependsOnOrdering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		// name is the subtest label; also distinguishes how the dependency reaches the
		// engine (explicit dependsOn vs. a URN placed in `dependencies` to simulate a
		// property reference — the gRPC proto flattens both into the same field).
		name string
		// prime, if non-nil, is a program run first to seed state (used by the replace case).
		prime func(t *testing.T, monitor *deploytest.ResourceMonitor) error
		// program is the program under test, called with the URN of any resource created by prime.
		program func(t *testing.T, monitor *deploytest.ResourceMonitor) error
		// diffReplace causes the provider to return a replace result from Diff, so a
		// subsequent update to src triggers replacement.
		diffReplace bool
	}{
		{
			name: "dependsOn new resource",
			program: func(t *testing.T, monitor *deploytest.ResourceMonitor) error {
				src, err := monitor.RegisterResource("pkgA:m:typA", "src", true, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"value": resource.NewProperty(true)},
				})
				require.NoError(t, err)
				_, err = monitor.Client().ReadResource(t.Context(), &pulumirpc.ReadResourceRequest{
					Type:         "pkgA:m:typA",
					Name:         "res",
					Id:           "existing-id",
					Dependencies: []string{string(src.URN)},
				})
				return err
			},
		},
		{
			name: "dependsOn resource being replaced",
			prime: func(t *testing.T, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "src", true, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"value": resource.NewProperty(true)},
				})
				return err
			},
			program: func(t *testing.T, monitor *deploytest.ResourceMonitor) error {
				// Changing src's input triggers Diff → replace (per diffReplace).
				src, err := monitor.RegisterResource("pkgA:m:typA", "src", true, deploytest.ResourceOptions{
					Inputs: resource.PropertyMap{"value": resource.NewProperty(false)},
				})
				require.NoError(t, err)
				_, err = monitor.Client().ReadResource(t.Context(), &pulumirpc.ReadResourceRequest{
					Type:         "pkgA:m:typA",
					Name:         "res",
					Id:           "existing-id",
					Dependencies: []string{string(src.URN)},
				})
				return err
			},
			diffReplace: true,
		},
	}

	for _, tc := range cases {
		for _, dryRun := range []bool{false, true} {
			mode := "update"
			if dryRun {
				mode = "preview"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				var mu sync.Mutex
				events := []string{}
				record := func(s string) {
					mu.Lock()
					defer mu.Unlock()
					events = append(events, s)
				}
				loaders := readDependsOnProviderLoader(record, tc.diffReplace)

				p := &lt.TestPlan{
					// Display tests replay only the program-under-test. Scenarios with a prime
					// step depend on seeded state that the display replay doesn't apply, so
					// their event streams don't match the generated snapshots. Skip those.
					Options: lt.TestUpdateOptions{T: t, SkipDisplayTests: tc.prime != nil},
				}
				project := p.GetProject()

				// If there's a prime step, run it first to seed state.
				var snap *deploy.Snapshot
				if tc.prime != nil {
					primeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
						return tc.prime(t, monitor)
					})
					p.Options.HostF = deploytest.NewPluginHostF(nil, nil, primeF, nil, nil, loaders...)
					var err error
					snap, err = lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
					require.NoError(t, err)
					// Drop the recorded create of the prime step.
					mu.Lock()
					events = events[:0]
					mu.Unlock()
				}

				programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					return tc.program(t, monitor)
				})
				p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

				_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, dryRun, p.BackendClient, nil)
				require.NoError(t, err)

				mu.Lock()
				defer mu.Unlock()
				if dryRun {
					// Preview: the dependency isn't actually created/replaced yet; invoking
					// the real provider's Read against its id would fail. The engine must
					// skip the provider call and signal unknown outputs to the SDK.
					assert.NotContains(t, events, "read:res",
						"Read should be skipped in preview when a dependency is being created or replaced (issue #16395)")
				} else {
					// Update: Read must run strictly after any create/replace of its dependencies.
					if assert.Contains(t, events, "read:res") &&
						assert.Contains(t, events, "create:src") {
						var createIdx, readIdx int
						for i, e := range events {
							if e == "create:src" {
								createIdx = i
							}
							if e == "read:res" {
								readIdx = i
							}
						}
						assert.Less(t, createIdx, readIdx,
							"Create of src must run before Read of res (dependsOn ordering, issue #16395)")
					}
				}
			})
		}
	}
}
