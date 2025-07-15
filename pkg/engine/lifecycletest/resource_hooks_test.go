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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

// By default hooks do not run on dry runs. Using the `OnDryRun` option makes them run on dry run.
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
		funTrue := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
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
		funFalse := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
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

func TestResourceHooksAfterCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(context.Context, plugin.CheckRequest) (plugin.CheckResponse, error) {
					return plugin.CheckResponse{
						Properties: resource.NewPropertyMapFromMap(map[string]any{
							"a": "A",
							"c": "C",
						}),
					}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("created-id-" + req.URN.Name())
					}
					props := resource.NewPropertyMapFromMap(map[string]any{
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

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Equal(t, map[string]any{"a": "A", "c": "C"}, newInputs.Mappable(), "receives the checked inputs")
			require.Nil(t, oldInputs, "there are no old inputs for creates")
			require.Equal(t, map[string]any{"a": "A", "b": "B", "c": "C"}, newOutputs.Mappable())
			require.Nil(t, oldOutputs, "there are no old outputs for creates")
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
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

// Before hooks that return an error cause the step to fail.
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

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Equal(t, map[string]any{"a": "A"}, newInputs.Mappable())
			require.Nil(t, oldInputs)
			require.Nil(t, oldOutputs)
			require.Nil(t, newOutputs)
			return errors.New("Oh no")
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
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
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("created-id-" + req.URN.Name())
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	createResource := true
	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Nil(t, newInputs, "deletes have no new inputs")
			require.Equal(t, map[string]any{"a": "A"}, oldInputs.Mappable(), "reveives the old inputs")
			require.Nil(t, newOutputs, "deletes have no new outputs")
			require.Equal(t, map[string]any{"a": "A"}, oldOutputs.Mappable(), "receives the old outputs")
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
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

func TestResourceHookComponentAfterDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// Copy the hooks from the construct options onto the resource options.
					binding := deploytest.ResourceHookBindings{}
					for _, h := range req.Options.ResourceHooks[resource.BeforeCreate] {
						binding.BeforeCreate = append(binding.BeforeCreate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterCreate] {
						binding.AfterCreate = append(binding.AfterCreate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.BeforeUpdate] {
						binding.BeforeUpdate = append(binding.BeforeUpdate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterUpdate] {
						binding.AfterUpdate = append(binding.AfterUpdate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.BeforeDelete] {
						binding.BeforeDelete = append(binding.BeforeDelete, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterDelete] {
						binding.AfterDelete = append(binding.AfterDelete, &deploytest.ResourceHook{Name: h})
					}
					opts := deploytest.ResourceOptions{
						ResourceHookBindings: binding,
						Inputs:               req.Inputs,
					}
					res, err := monitor.RegisterResource("pkgA:m:typB", req.Name, false, opts)
					require.NoError(t, err)
					outs := resource.NewPropertyMapFromMap(map[string]any{"outA": "outA"})
					err = monitor.RegisterResourceOutputs(res.URN, outs)
					require.NoError(t, err)
					return plugin.ConstructResponse{
						URN:     res.URN,
						Outputs: outs,
					}, nil
				},
			}, nil
		}),
	}

	createResource := true
	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typB"))
			require.Nil(t, newInputs, "deletes have no new inputs")
			// Note that receiving the oldInputs is SDK specific.
			// In our "SDK" for this test, `ConstructF` above passes inputs through to the resource.
			// However for example the Go SDK does not pass through inputs in RegisterComponentResource.
			require.Equal(t, map[string]any{"a": "A"}, oldInputs.Mappable(), "reveives the old inputs")
			require.Nil(t, newOutputs, "deletes have no new outputs")
			require.Equal(t, map[string]any{"outA": "outA"}, oldOutputs.Mappable(), "receives the old outputs")
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Remote: true,
				Inputs: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
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
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("created-id-" + req.URN.Name())
					}
					return plugin.CreateResponse{
						ID: id,
						Properties: resource.NewPropertyMapFromMap(map[string]any{
							"a": "A",
						}),
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	createResource := true
	hookCalled := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Nil(t, newInputs, "deletes have no new inputs")
			require.Equal(t, map[string]any{"a": "A"}, oldInputs.Mappable(), "receives old inputs")
			require.Nil(t, newOutputs, "deletes have no new outputs")
			require.Equal(t, map[string]any{"a": "A"}, oldOutputs.Mappable(), "receives old outputs")
			return errors.New("Oh no")
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]any{"a": "A"}),
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

// Test that we run a BeforeUpdate hook that's coming in on the *new* state, but
// pass the values of the *old* state to the hook callback.
func TestResourceHookBeforeUpdate(t *testing.T) {
	t.Parallel()

	createOutputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo":  "bar",
		"frob": "baz",
		"baz":  24,
	})
	updateOutputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo":  "bar",
		"frob": "updated",
		"baz":  24,
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("created-id-" + req.URN.Name())
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: createOutputs,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{
						Properties: updateOutputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	isUpdate := false
	hookCalled := false
	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo":  "bar",
		"frob": "baz",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		// We create the resource with this hook as a `beforeUpdate` hook, and
		// then update the resource to use a different hook. We expect this hook
		// to not be called during the update.
		shouldNotBeCalled := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			require.Fail(t, "Hook should not be called")
			return nil
		}
		shouldNotBeCalledHook, err := deploytest.NewHook(monitor, callbacks, "shouldNotBeCalled", shouldNotBeCalled, true)
		require.NoError(t, err)

		shouldBeCalled := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Equal(t, map[string]any{
				"foo":  "bar",
				"frob": "updated",
			}, newInputs.Mappable(), "Hook receieves the new inputs")
			require.Equal(t, map[string]any{
				"foo":  "bar",
				"frob": "baz",
			}, oldInputs.Mappable(), "Hook receieves the old inputs")
			require.Equal(t, map[string]any{
				"foo":  "bar",
				"frob": "baz",
				"baz":  float64(24),
			}, oldOutputs.Mappable(), "Hook receieves the old outputs")
			require.Nil(t, newOutputs, "there are no new outputs for before update hooks")
			return nil
		}
		shouldBeCalledHook, err := deploytest.NewHook(monitor, callbacks, "shouldBeCalled", shouldBeCalled, true)
		require.NoError(t, err)

		// On the first run through the program, we'll register `shouldNotBeCalledHook` as a BeforeUpdate hook
		// for the resource.
		hooks := []*deploytest.ResourceHook{shouldNotBeCalledHook}
		if isUpdate {
			// On the second run through, we switch to another hook. We expect this hook to be called during
			// the update.
			hooks = []*deploytest.ResourceHook{shouldBeCalledHook}
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				BeforeUpdate: hooks,
			},
		})
		require.NoError(t, err)

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

	// change the inputs
	inputs = resource.NewPropertyMapFromMap(map[string]any{
		"foo":  "bar",
		"frob": "updated",
	})
	// and use the new hook
	isUpdate = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.True(t, hookCalled)
}

func TestResourceHookBeforeUpdateError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("created-id-" + req.URN.Name())
					}
					return plugin.CreateResponse{
						ID:         id,
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
		}),
	}

	isUpdate := false
	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			require.Equal(t, map[string]any{"foo": "updated"}, newInputs.Mappable(), "receives new inputs")
			require.Equal(t, map[string]any{"foo": "bar"}, oldInputs.Mappable(), "receives old inputs")
			require.Equal(t, map[string]any{"foo": "bar"}, oldOutputs.Mappable(), "receives old outputs")
			require.Nil(t, newOutputs, "there are no new outputs")
			return errors.New("this hook returns an error")
		}
		hook, err := deploytest.NewHook(monitor, callbacks, "hook", hookFun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				BeforeUpdate: []*deploytest.ResourceHook{hook},
			},
		})

		if isUpdate {
			require.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			return err
		}

		require.NoError(t, err)
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

	// change the inputs
	inputs = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "updated",
	})
	isUpdate = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "before hook \"hook\" failed: this hook returns an error")
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.Equal(t, snap.Resources[1].Outputs.Mappable(), map[string]any{
		"foo": "bar",
	}, "the resource was not updated")
}

func TestResourceHookDeleteCalledOnDestroyRunProgram(t *testing.T) {
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

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ResourceHookBindings: deploytest.ResourceHookBindings{
				AfterDelete: []*deploytest.ResourceHook{myHook},
			},
		})
		require.NoError(t, err)

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

	// Run a destroy with the program
	snap, err = lt.TestOp(DestroyV2).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.True(t, hookCalled)
	require.Len(t, snap.Resources, 0)
}

func TestResourceHookDeleteErrorWhenNoRunProgram(t *testing.T) {
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

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			hookCalled = true
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typA"))
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ResourceHookBindings: deploytest.ResourceHookBindings{
				BeforeDelete: []*deploytest.ResourceHook{myHook},
			},
		})
		require.NoError(t, err)

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

	// Run a destroy without the program
	snap, err = lt.TestOp(Destroy).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "You must run with the `--run-program` flag to use delete hooks during destroy.")
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.False(t, hookCalled)
}

func TestResourceHookComponent(t *testing.T) {
	t.Parallel()

	hookCalled := false

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// Copy the hooks from the construct options onto the resource options.
					binding := deploytest.ResourceHookBindings{}
					for _, h := range req.Options.ResourceHooks[resource.BeforeCreate] {
						binding.BeforeCreate = append(binding.BeforeCreate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterCreate] {
						binding.AfterCreate = append(binding.AfterCreate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.BeforeUpdate] {
						binding.BeforeUpdate = append(binding.BeforeUpdate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterUpdate] {
						binding.AfterUpdate = append(binding.AfterUpdate, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.BeforeDelete] {
						binding.BeforeDelete = append(binding.BeforeDelete, &deploytest.ResourceHook{Name: h})
					}
					for _, h := range req.Options.ResourceHooks[resource.AfterDelete] {
						binding.AfterDelete = append(binding.AfterDelete, &deploytest.ResourceHook{Name: h})
					}
					opts := deploytest.ResourceOptions{ResourceHookBindings: binding}
					res, err := monitor.RegisterResource("pkgA:m:typB", req.Name, false, opts)
					require.NoError(t, err)
					outs := resource.PropertyMap{}
					err = monitor.RegisterResourceOutputs(res.URN, outs)
					require.NoError(t, err)
					return plugin.ConstructResponse{
						URN:     res.URN,
						Outputs: outs,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap,
		) error {
			require.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resA"))
			require.Equal(t, name, "resA")
			require.Equal(t, typ, tokens.Type("pkgA:m:typB"))
			hookCalled = true
			return nil
		}
		myHook, err := deploytest.NewHook(monitor, callbacks, "myHook", fun, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				AfterCreate: []*deploytest.ResourceHook{
					myHook,
				},
			},
		})
		require.NoError(t, err)

		err = monitor.SignalAndWaitForShutdown(context.Background())
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.Equal(t, snap.Resources[0].URN.Name(), "default")
	require.Equal(t, snap.Resources[1].URN.Name(), "resA")
	require.True(t, hookCalled)
}
