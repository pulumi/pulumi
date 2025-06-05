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
	"fmt"
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
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type hookFunc func(ctx context.Context, urn resource.URN, id resource.ID, outputs resource.PropertyMap) error

func prepareHook(callbacks *deploytest.CallbackServer, name string, f hookFunc) (*pulumirpc.Callback, error) {
	wrapped := func(request []byte) (proto.Message, error) {
		var req pulumirpc.LifecycleHookRequest
		err := proto.Unmarshal(request, &req)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling request: %w", err)
		}
		outs, err := plugin.UnmarshalProperties(req.Outputs, plugin.MarshalOptions{
			KeepUnknowns:     true,
			KeepSecrets:      true,
			KeepResources:    true,
			KeepOutputValues: true,
		})
		if err != nil {
			return nil, err
		}
		if err := f(context.Background(), resource.URN(req.Urn), resource.ID(req.Id), outs); err != nil {
			return &pulumirpc.LifecycleHookResponse{
				Error: err.Error(),
			}, nil
		}
		return &pulumirpc.LifecycleHookResponse{}, nil
	}
	callback, err := callbacks.AllocateWithToken(wrapped, name)
	if err != nil {
		return nil, err
	}
	return callback, nil
}

func TestLifecycleHooks(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("id")
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
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

		// Create the hook and register it
		fun := func(ctx context.Context, urn resource.URN, id resource.ID, outputs resource.PropertyMap) error {
			hookCalled = true
			assert.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			if info.DryRun {
				assert.Empty(t, id)
			} else {
				assert.Equal(t, resource.ID("id"), id)
			}
			assert.Equal(t, outputs, resource.NewPropertyMapFromMap(map[string]interface{}{
				"foo": "bar",
			}))
			return nil
		}
		callback, err := prepareHook(callbacks, "myHook", fun)
		require.NoError(t, err)
		hook := deploytest.LifecycleHook{
			Callback: callback,
		}
		err = monitor.RegisterLifecycleHook(context.Background(), callback)
		assert.NoError(t, err)

		// Register a resource with an after create hook
		inputs := resource.NewPropertyMapFromMap(map[string]interface{}{
			"foo": "bar",
		})
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			LifecycleHookBindings: deploytest.LifecycleHookBindings{
				AfterCreate: []deploytest.LifecycleHook{hook},
			},
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.True(t, hookCalled)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

func TestLifecycleHookBeforeCreateError(t *testing.T) {
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

		// Create the hook and register it
		fun := func(ctx context.Context, urn resource.URN, id resource.ID, outputs resource.PropertyMap) error {
			hookCalled = true
			assert.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return errors.New("Oh no")
		}
		callback, err := prepareHook(callbacks, "myHook", fun)
		require.NoError(t, err)
		hook := deploytest.LifecycleHook{
			Callback: callback,
		}
		err = monitor.RegisterLifecycleHook(context.Background(), callback)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			LifecycleHookBindings: deploytest.LifecycleHookBindings{
				BeforeCreate: []deploytest.LifecycleHook{hook},
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

			assert.True(t, sawFailure, "There should be an error diagnostic for `resAB`")
			return err
		},
	}}
	snap := p.Run(t, nil)
	assert.True(t, hookCalled)
	assert.Len(t, snap.Resources, 1)
}

func TestLifecycleHookAfterDelete(t *testing.T) {
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

		// Create the hook and register it
		fun := func(ctx context.Context, urn resource.URN, id resource.ID, outputs resource.PropertyMap) error {
			hookCalled = true
			assert.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return nil
		}
		callback, err := prepareHook(callbacks, "myHook", fun)
		require.NoError(t, err)
		hook := deploytest.LifecycleHook{
			Callback: callback,
		}
		err = monitor.RegisterLifecycleHook(context.Background(), callback)
		assert.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				LifecycleHookBindings: deploytest.LifecycleHookBindings{
					AfterDelete: []deploytest.LifecycleHook{hook},
				},
			})
			assert.NoError(t, err)
		}

		err = monitor.WaitForShutdown(context.Background())
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
	require.NoError(t, err)
	assert.NotNil(t, snap)
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

func TestLifecycleHookBeforeDeleteError(t *testing.T) {
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

		// Create the hook and register it
		fun := func(ctx context.Context, urn resource.URN, id resource.ID, outputs resource.PropertyMap) error {
			hookCalled = true
			assert.Equal(t, urn, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"))
			return errors.New("Oh no")
		}
		callback, err := prepareHook(callbacks, "myHook", fun)
		require.NoError(t, err)
		hook := deploytest.LifecycleHook{
			Callback: callback,
		}
		err = monitor.RegisterLifecycleHook(context.Background(), callback)
		assert.NoError(t, err)

		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				LifecycleHookBindings: deploytest.LifecycleHookBindings{
					BeforeDelete: []deploytest.LifecycleHook{hook},
				},
			})
			assert.NoError(t, err)
		}

		err = monitor.WaitForShutdown(context.Background())
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
	require.NoError(t, err)
	assert.NotNil(t, snap)
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

// TODO: more hook errors tests:
// * errors in after hooks should emit a warning diagnostic
// * add tests running Destroy (with --run-program)
