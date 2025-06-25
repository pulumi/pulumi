// Copyright 2016-2025, Pulumi Corporation.
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
	"errors"
	"strings"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestResourceHooks(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(context.Context, plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{
						Properties: resource.NewPropertyMapFromMap(map[string]interface{}{
							"a": "A",
							"c": "C",
						}),
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("id")
					}
					props := resource.NewPropertyMapFromMap(map[string]interface{}{
						"a": "A",
						"b": "B",
						"c": "C",
					})
					return plugin.CreateResponse{
						ID:         id,
						Properties: props,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			hookCalled = true
			expectedInputs := map[string]any{
				"a": "A",
				"c": "C",
			}
			require.Equal(t, expectedInputs, inputs.Mappable(), "Hook receieves the checked inputs")
			expectedOutputs := map[string]any{
				"a": "A",
				"b": "B",
				"c": "C",
			}
			require.Equal(t, expectedOutputs, outputs.Mappable())
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{
				"a": "A",
			}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				AfterCreate: []*deploytest.ResourceHook{myHook},
			},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	require.True(t, hookCalled)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestResourceHookDryRun(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	hookTrueCalledOnDryRun := false
	hookTrueCalled := false
	hookFalseCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		// Create hook that runs on a dry run (and non-dry run)
		funTrue := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			if info.DryRun {
				hookTrueCalledOnDryRun = true
			} else {
				hookTrueCalled = true
			}
			return nil
		}
		myHookTrue, err := deploytest.NewHook(monitor, callbacks, "myHookTrue", funTrue, true)
		require.NoError(t, err)

		// Create hook that does not run on a dry run
		funFalse := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			if info.DryRun {
				require.Fail(t, "The hook should not have been called")
			} else {
				hookFalseCalled = true
			}
			return nil
		}
		myHookFalse, err := deploytest.NewHook(monitor, callbacks, "myHookFalse", funFalse, false)
		require.NoError(t, err)

		// Register a resource with both hooks
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ResourceHookBindings: deploytest.ResourceHookBindings{
				AfterCreate: []*deploytest.ResourceHook{myHookTrue, myHookFalse},
			},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	require.True(t, hookTrueCalledOnDryRun, "hook true should have been called on dry run")
	require.True(t, hookTrueCalled, "hook true should have been called")
	require.True(t, hookFalseCalled, "hook false should have been called")
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestResourceHookBeforeCreateError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return errors.New("Oh no")
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ResourceHookBindings: deploytest.ResourceHookBindings{
				BeforeCreate: []*deploytest.ResourceHook{myHook},
			},
		})
		require.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	p.Steps = []lt.TestStep{{
		Op:            Update,
		SkipPreview:   true,
		ExpectFailure: true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			sawFailure := false
			for _, evt := range evts {
				if evt.Type == DiagEvent {
					e := evt.Payload().(DiagEventPayload)
					sawFailure = strings.Contains(e.Message, "hook \"myHook\" failed: Oh no") &&
						e.Severity == diag.Error && e.URN.Name() == "resA"
					if sawFailure {
						break
					}
				}
			}

			require.True(t, sawFailure, "There should be an error diagnostic for `resAB`")
			return err
		},
	}}
	snap := p.Run(t, nil)
	require.True(t, hookCalled)
	require.Len(t, snap.Resources, 1)
}

func TestResourceHookAfterDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	createResource := true
	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					AfterDelete: []*deploytest.ResourceHook{myHook},
				},
			})
			require.NoError(t, err)
		}

		err = monitor.SignalAndWaitForShutdown(context.Background())
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.False(t, hookCalled)

	// Now run an update without the resource, its delete hook should be called.
	createResource = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.True(t, hookCalled)
	require.Len(t, snap.Resources, 0)
}

func TestResourceHookBeforeDeleteError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	createResource := true
	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, inputs resource.PropertyMap,
			outputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return errors.New("Oh no")
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					BeforeDelete: []*deploytest.ResourceHook{myHook},
				},
			})
			require.NoError(t, err)
		}

		err = monitor.SignalAndWaitForShutdown(context.Background())
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	project := p.GetProject()

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.False(t, hookCalled)

	// Now run an update without the resource, the beforeDelete hook should be called and prevent deletion
	createResource = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "hook \"myHook\" failed: Oh no")
	require.True(t, hookCalled)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
}
