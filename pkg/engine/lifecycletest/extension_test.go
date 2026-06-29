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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestExtensionParameterizedProvider exercises the full extension-parameterization
// flow end-to-end against a fake provider:
//
//   - The program registers two extensions on the same base provider via
//     RegisterPackage and creates one resource per extension.
//   - The fake provider's Parameterize is called twice (cumulatively, on the same
//     plugin instance) — once per extension blob.
//   - The resulting snapshot persists both extension blobs in snap.Extensions and
//     records each resource's ExtensionRef.
//   - A second Update against the same snapshot — without the program re-supplying
//     the blobs — exercises load-time rehydration: the engine pulls blobs from
//     state and replays Parameterize on the fresh plugin instance before any
//     resource ops touch it.
func TestExtensionParameterizedProvider(t *testing.T) {
	t.Parallel()

	// Track parameterize calls across the test lifetime so we can assert on
	// what blobs reached the provider, and from which run.
	type paramCall struct {
		name, version string
		value         []byte
	}
	var paramLock sync.Mutex
	var paramCalls []paramCall

	recordParameterize := func(value *plugin.ParameterizeValue) {
		paramLock.Lock()
		defer paramLock.Unlock()
		paramCalls = append(paramCalls, paramCall{
			name:    value.Name,
			version: value.Version.String(),
			value:   append([]byte(nil), value.Value...),
		})
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ParameterizeF: func(
					_ context.Context, req plugin.ParameterizeRequest,
				) (plugin.ParameterizeResponse, error) {
					value := req.Parameters.(*plugin.ParameterizeValue)
					recordParameterize(value)
					return plugin.ParameterizeResponse{
						Name:    value.Name,
						Version: value.Version,
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					paramLock.Lock()
					witnessed := len(paramCalls)
					paramLock.Unlock()
					assert.NotZero(t, witnessed,
						"Parameterize must be witnessed before Create on the extension plugin")
					return plugin.CreateResponse{
						ID:         resource.ID("id-" + req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					paramLock.Lock()
					witnessed := len(paramCalls)
					paramLock.Unlock()
					assert.NotZero(t, witnessed,
						"Parameterize must be witnessed before Read on the extension plugin")
					state := req.State
					if state == nil {
						state = resource.PropertyMap{}
					}
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{ID: req.ID, Outputs: state, Inputs: req.Inputs},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	// extProgram registers two extensions on the base provider with one resource each,
	// then re-registers ext-a byte-for-byte (refADup) to exercise ref stability.
	var refA, refB, refADup string
	extProgram := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		refA, err = monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil, &pulumirpc.Parameterization{
			Name: "ext-a", Version: "1.0.0", Value: []byte("blob-a"),
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{PackageRef: refA})
		require.NoError(t, err)

		refB, err = monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil, &pulumirpc.Parameterization{
			Name: "ext-b", Version: "1.0.0", Value: []byte("blob-b"),
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{PackageRef: refB})
		require.NoError(t, err)

		refADup, err = monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil, &pulumirpc.Parameterization{
			Name: "ext-a", Version: "1.0.0", Value: []byte("blob-a"),
		})
		require.NoError(t, err)
		return nil
	})

	// noopProgram registers nothing, forcing the engine to drive existing extension
	// resources purely from state.
	noopProgram := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})

	paramNames := func(calls []paramCall) []string {
		out := make([]string, len(calls))
		for i, c := range calls {
			out[i] = c.name
		}
		return out
	}

	// Phases run in order against one evolving snapshot; each subtest name states the
	// property under test. paramCalls is reset before each phase so its check sees only
	// that phase's Parameterize calls.
	phases := []struct {
		name    string
		op      lt.TestOp
		program deploytest.LanguageRuntimeFactory
		check   func(t *testing.T, snap *deploy.Snapshot, params []paramCall)
	}{
		{
			name:    "create_parameterizes_provider_then_persists_blobs_and_refs",
			op:      lt.TestOp(Update),
			program: extProgram,
			check: func(t *testing.T, snap *deploy.Snapshot, params []paramCall) {
				require.Len(t, params, 2, "create should parameterize once per extension")
				assert.Equal(t, "ext-a", params[0].name)
				assert.Equal(t, []byte("blob-a"), params[0].value)
				assert.Equal(t, "ext-b", params[1].name)
				assert.Equal(t, []byte("blob-b"), params[1].value)

				assert.Equal(t, refA, refADup, "a byte-identical extension must yield the same ref")
				assert.NotEqual(t, refA, refB, "different extensions must produce different refs")

				require.Len(t, snap.Extensions, 2, "snapshot should carry both extension blobs")
				assert.Equal(t, []byte("blob-a"), snap.Extensions[apitype.ExtensionRef(refA)].Value)
				assert.Equal(t, []byte("blob-b"), snap.Extensions[apitype.ExtensionRef(refB)].Value)

				var resA, resB *resource.State
				for _, r := range snap.Resources {
					switch r.URN.Name() {
					case "resA":
						resA = r
					case "resB":
						resB = r
					}
				}
				require.NotNil(t, resA, "resA missing from snapshot")
				require.NotNil(t, resB, "resB missing from snapshot")
				assert.Equal(t, apitype.ExtensionRef(refA), resA.ExtensionRef)
				assert.Equal(t, apitype.ExtensionRef(refB), resB.ExtensionRef)
			},
		},
		{
			name:    "refresh_rehydrates_extensions_from_state_without_the_program",
			op:      lt.TestOp(Refresh),
			program: noopProgram,
			check: func(t *testing.T, _ *deploy.Snapshot, params []paramCall) {
				require.Len(t, params, 2, "refresh must replay both extensions from state")
				assert.ElementsMatch(t, []string{"ext-a", "ext-b"}, paramNames(params))
			},
		},
		{
			name:    "no_change_update_parameterizes_each_extension_once_not_per_source",
			op:      lt.TestOp(Update),
			program: extProgram,
			check: func(t *testing.T, _ *deploy.Snapshot, params []paramCall) {
				require.Len(t, params, 2,
					"a no-change up must parameterize once per extension, not once per source")
				assert.ElementsMatch(t, []string{"ext-a", "ext-b"}, paramNames(params))
			},
		},
	}

	var snap *deploy.Snapshot
	for _, ph := range phases {
		ok := t.Run(ph.name, func(t *testing.T) {
			paramLock.Lock()
			paramCalls = nil
			paramLock.Unlock()

			hostF := deploytest.NewPluginHostF(nil, nil, ph.program, nil, nil, loaders...)
			p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

			next, err := ph.op.RunStep(
				p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "")
			require.NoError(t, err)
			require.NotNil(t, next)

			paramLock.Lock()
			params := append([]paramCall(nil), paramCalls...)
			paramLock.Unlock()

			ph.check(t, next, params)
			snap = next
		})
		require.True(t, ok, "phase %q failed; later phases build on its snapshot", ph.name)
	}
}

func TestExtensionParameterizedProviderDeleteParameterizesFromState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		deleteOp lt.TestOp
	}{
		{"destroy", lt.TestOp(Destroy)},
		{"remove_on_update", lt.TestOp(Update)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var mu sync.Mutex
			parameterizeCount := 0

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						ParameterizeF: func(
							_ context.Context, req plugin.ParameterizeRequest,
						) (plugin.ParameterizeResponse, error) {
							value := req.Parameters.(*plugin.ParameterizeValue)
							mu.Lock()
							parameterizeCount++
							mu.Unlock()
							return plugin.ParameterizeResponse{Name: value.Name, Version: value.Version}, nil
						},
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{
								ID:         resource.ID("id-" + req.URN.Name()),
								Properties: req.Properties,
								Status:     resource.StatusOK,
							}, nil
						},
						DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							parameterized := parameterizeCount > 0
							mu.Unlock()
							assert.True(t, parameterized,
								"provider must be parameterized before an extension resource is deleted")
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			createResource := deploytest.NewLanguageRuntimeF(
				func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					ref, err := monitor.RegisterPackage("pkgA", "1.0.0", "", nil, nil, &pulumirpc.Parameterization{
						Name: "ext-a", Version: "1.0.0", Value: []byte("blob-a"),
					})
					require.NoError(t, err)
					_, err = monitor.RegisterResource(
						"pkgA:m:typA", "resA", true, deploytest.ResourceOptions{PackageRef: ref})
					require.NoError(t, err)
					return nil
				})

			hostF := deploytest.NewPluginHostF(nil, nil, createResource, nil, nil, loaders...)
			p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
			snap, err := lt.TestOp(Update).
				RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "")
			require.NoError(t, err)
			require.NotNil(t, snap)

			mu.Lock()
			parameterizeCount = 0
			mu.Unlock()

			noProgram := deploytest.NewLanguageRuntimeF(
				func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error { return nil })
			hostF2 := deploytest.NewPluginHostF(nil, nil, noProgram, nil, nil, loaders...)
			p2 := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF2, SkipDisplayTests: true}}
			_, err = c.deleteOp.
				RunStep(p2.GetProject(), p2.GetTarget(t, snap), p2.Options, false, p2.BackendClient, nil, "")
			require.NoError(t, err)

			mu.Lock()
			parameterized := parameterizeCount > 0
			mu.Unlock()
			require.True(t, parameterized,
				"a from-state delete must parameterize the provider from state")
		})
	}
}
