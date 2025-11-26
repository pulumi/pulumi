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
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

func TestErrorHookNotCalledOnSuccessCreate(t *testing.T) {
	t.Parallel()

	hookCalled := false
	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook,
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
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	require.False(t, hookCalled, "error hook should not be called when operation succeeds")
	require.Len(t, snap.Resources, 2)
}

func TestErrorHookNotCalledOnSuccessUpdate(t *testing.T) {
	t.Parallel()

	hookCalled := false
	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorUpdate: errorHook,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	inputs = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "updated",
	})

	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)

	require.False(t, hookCalled, "error hook should not be called when operation succeeds")
	require.Len(t, snap.Resources, 2)
}

func TestErrorHookNotCalledOnSuccessDelete(t *testing.T) {
	t.Parallel()

	hookCalled := false

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorDelete: errorHook,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	p.Steps = []lt.TestStep{{Op: DestroyV2}}
	snap = p.Run(t, snap)

	require.False(t, hookCalled, "error hook should not be called when operation succeeds")
	require.Len(t, snap.Resources, 0)
}

func TestErrorHookFailWithoutRetryCreate(t *testing.T) {
	t.Parallel()

	hookCalled := false
	var hookErrors []error

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("create failed")
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			hookErrors = errors
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook,
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

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")

	require.Error(t, err, "operation should fail")
	require.True(t, hookCalled, "error hook should be called")
	require.Len(t, hookErrors, 1, "hook should receive one error")
}

func TestErrorHookFailWithoutRetryUpdate(t *testing.T) {
	t.Parallel()

	hookCalled := false
	var hookErrors []error

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{}, errors.New("update failed")
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			hookErrors = errors
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorUpdate: errorHook,
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

	snap, createErr := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, createErr)

	inputs = resource.NewPropertyMapFromMap(map[string]any{"foo": "updated"})
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.Error(t, err, "operation should fail")
	require.True(t, hookCalled, "error hook should be called")
	require.Len(t, hookErrors, 1, "hook should receive one error")
}

func TestErrorHookFailWithoutRetryDelete(t *testing.T) {
	t.Parallel()

	hookCalled := false
	var hookErrors []error

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("delete failed")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCalled = true
			hookErrors = errors
			return false, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorDelete: errorHook,
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

	snap, createErr := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, createErr)
	_, err := lt.TestOp(DestroyV2).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.Error(t, err, "operation should fail")
	require.True(t, hookCalled, "error hook should be called")
	require.Len(t, hookErrors, 1, "hook should receive one error")
}

func TestErrorHookMaxRetriesCreate(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{}, errors.New("create failed")
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, hookCallCount, "hook should receive all errors so far")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook,
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

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")

	require.Error(t, err, "operation should fail after max retries")
	require.Contains(t, err.Error(), "maximum retry limit (100) exceeded", "should fail with retry limit message")

	require.Equal(t, 100, hookCallCount, "hook should be called exactly 100 times")
}

func TestErrorHookMaxRetriesUpdate(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{}, errors.New("update failed")
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, hookCallCount, "hook should receive all errors so far")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorUpdate: errorHook,
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

	// First create
	snap, createErr := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, createErr)
	inputs = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "updated",
	})
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.Error(t, err, "operation should fail after max retries")
	require.Contains(t, err.Error(), "maximum retry limit (100) exceeded", "should fail with retry limit message")

	require.Equal(t, 100, hookCallCount, "hook should be called exactly 100 times")
}

func TestErrorHookMaxRetriesDelete(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("delete failed")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, hookCallCount, "hook should receive all errors so far")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorDelete: errorHook,
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

	snap, createErr := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, createErr)
	_, err := lt.TestOp(DestroyV2).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.Error(t, err, "operation should fail after max retries")
	require.Contains(t, err.Error(), "maximum retry limit (100) exceeded", "should fail with retry limit message")

	require.Equal(t, 100, hookCallCount, "hook should be called exactly 100 times")
}

func TestErrorHookRetryOnceThenSucceedCreate(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			attemptCount := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					attemptCount++
					if attemptCount == 1 {
						return plugin.CreateResponse{}, errors.New("create failed")
					}
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, 1, "hook should receive one error on first call")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")

	require.NoError(t, err, "operation should succeed after retry")
	require.Equal(t, 1, hookCallCount, "hook should be called exactly once")
	require.Len(t, snap.Resources, 2)
}

func TestErrorHookRetryOnceThenSucceedUpdate(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			attemptCount := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					attemptCount++
					if attemptCount == 1 {
						return plugin.UpdateResponse{}, errors.New("update failed")
					}
					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, 1, "hook should receive one error on first call")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorUpdate: errorHook,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	inputs = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "updated",
	})

	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.NoError(t, err, "operation should succeed after retry")
	require.Equal(t, 1, hookCallCount, "hook should be called exactly once")
	require.Len(t, snap.Resources, 2) // Default provider + resA
}

func TestErrorHookRetryOnceThenSucceedDelete(t *testing.T) {
	t.Parallel()

	hookCallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			attemptCount := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         resource.ID("123"),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					attemptCount++
					if attemptCount == 1 {
						return plugin.DeleteResponse{}, errors.New("delete failed")
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hookFun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hookCallCount++

			require.Len(t, errors, 1, "hook should receive one error on first call")
			return true, nil
		}
		errorHook, err := deploytest.NewErrorHook(monitor, callbacks, "myErrorHook", hookFun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorDelete: errorHook,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	snap, err = lt.TestOp(DestroyV2).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.NoError(t, err, "operation should succeed after retry")
	require.Equal(t, 1, hookCallCount, "hook should be called exactly once")
	require.Len(t, snap.Resources, 0)
}

func TestErrorHookTwoSeparateResources(t *testing.T) {
	t.Parallel()

	hook1CallCount := 0
	hook2CallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			attemptCount1 := 0
			attemptCount2 := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Name() == "resA" {
						attemptCount1++
						if attemptCount1 == 1 {
							return plugin.CreateResponse{}, errors.New("resA create failed")
						}
					} else if req.URN.Name() == "resB" {
						attemptCount2++
						if attemptCount2 == 1 {
							return plugin.CreateResponse{}, errors.New("resB create failed")
						}
					}
					return plugin.CreateResponse{
						ID:         resource.ID(req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook1Fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hook1CallCount++
			require.Equal(t, "resA", name, "hook1 should only be called for resA")
			return true, nil
		}
		errorHook1, err := deploytest.NewErrorHook(monitor, callbacks, "hook1", hook1Fun)
		require.NoError(t, err)

		hook2Fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hook2CallCount++

			require.Equal(t, "resB", name, "hook2 should only be called for resB")
			return true, nil
		}

		errorHook2, err := deploytest.NewErrorHook(monitor, callbacks, "hook2", hook2Fun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook1,
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook2,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 3)

	hook1Count := hook1CallCount
	hook2Count := hook2CallCount

	require.Equal(t, 1, hook1Count, "hook1 should be called exactly once for resA")
	require.Equal(t, 1, hook2Count, "hook2 should be called exactly once for resB")
}

func TestErrorHookRetryDoesNotAffectSubsequent(t *testing.T) {
	t.Parallel()

	hook1CallCount := 0
	hook2CallCount := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			attemptCount1 := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Name() == "resA" {
						attemptCount1++
						if attemptCount1 == 1 {
							return plugin.CreateResponse{}, errors.New("resA create failed")
						}
					}

					return plugin.CreateResponse{
						ID:         resource.ID(req.URN.Name()),
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook1Fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hook1CallCount++

			require.Equal(t, "resA", name, "hook1 should only be called for resA")
			return true, nil
		}
		errorHook1, err := deploytest.NewErrorHook(monitor, callbacks, "hook1", hook1Fun)
		require.NoError(t, err)

		hook2Fun := func(ctx context.Context, urn resource.URN, id resource.ID, name string, typ tokens.Type,
			newInputs, oldInputs, newOutputs, oldOutputs resource.PropertyMap, errors []error,
		) (bool, error) {
			hook2CallCount++

			require.Fail(t, "hook2 should never be called since resB succeeds")
			return false, nil
		}
		errorHook2, err := deploytest.NewErrorHook(monitor, callbacks, "hook2", hook2Fun)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook1,
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			ErrorHookBindings: deploytest.ErrorHookBindings{
				OnErrorCreate: errorHook2,
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 3)

	hook1Count := hook1CallCount
	hook2Count := hook2CallCount

	require.Equal(t, 1, hook1Count, "hook1 should be called exactly once for resA")
	require.Equal(t, 0, hook2Count, "hook2 should never be called since resB succeeds")
}
