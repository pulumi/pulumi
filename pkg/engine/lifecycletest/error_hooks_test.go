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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestErrorHooks_OperationIdentifierAndMultipleHooks_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			createCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					createCalls++
					if createCalls == 1 && !req.Preview {
						return plugin.CreateResponse{
								ID:     resource.ID("partial-id-" + req.URN.Name()),
								Status: resource.StatusPartialFailure,
							},
							errors.New("create failed")
					}
					return plugin.CreateResponse{
						ID:         resource.ID("created-id-" + req.URN.Name()),
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var hook1Called, hook2Called bool
	var hook1Op, hook2Op string

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook1, err := deploytest.NewHook(monitor, callbacks, "hook1",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook1Called = true
				hook1Op = failedOperation
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		hook2, err := deploytest.NewHook(monitor, callbacks, "hook2",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook2Called = true
				hook2Op = failedOperation
				require.NotEmpty(t, errs)
				return false, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{hook1, hook2},
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
	require.NotNil(t, snap)

	require.True(t, hook1Called)
	require.True(t, hook2Called)
	require.Equal(t, "create", hook1Op)
	require.Equal(t, "create", hook2Op)
	require.Len(t, snap.Resources, 2)
}

func TestErrorHooks_OperationIdentifierAndMultipleHooks_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			updateCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					updateCalls++
					if updateCalls == 1 && !req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	var hook1Called, hook2Called bool
	var hook1Op, hook2Op string

	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook1, err := deploytest.NewHook(monitor, callbacks, "hook1",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook1Called = true
				hook1Op = failedOperation
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		hook2, err := deploytest.NewHook(monitor, callbacks, "hook2",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook2Called = true
				hook2Op = failedOperation
				require.NotEmpty(t, errs)
				return false, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{hook1, hook2},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.False(t, hook1Called)
	require.False(t, hook2Called)

	// Update
	isUpdate = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.True(t, hook1Called)
	require.True(t, hook2Called)
	require.Equal(t, "update", hook1Op)
	require.Equal(t, "update", hook2Op)
}

func TestErrorHooks_OperationIdentifierAndMultipleHooks_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			deleteCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					if deleteCalls == 1 {
						return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	var hook1Called, hook2Called bool
	var hook1Op, hook2Op string

	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hook1, err := deploytest.NewHook(monitor, callbacks, "hook1",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook1Called = true
				hook1Op = failedOperation
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		hook2, err := deploytest.NewHook(monitor, callbacks, "hook2",
			func(_ context.Context, urn resource.URN, _ resource.ID, name string, typ tokens.Type,
				_, _, _, _ resource.PropertyMap, failedOperation string, errs []string,
			) (bool, error) {
				require.Equal(t, "resA", name)
				require.Equal(t, tokens.Type("pkgA:m:typA"), typ)
				require.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), urn)
				hook2Called = true
				hook2Op = failedOperation
				require.NotEmpty(t, errs)
				return false, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{hook1, hook2},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	require.False(t, hook1Called)
	require.False(t, hook2Called)

	// Delete
	programCreate = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.True(t, hook1Called)
	require.True(t, hook2Called)
	require.Equal(t, "delete", hook1Op)
	require.Equal(t, "delete", hook2Op)
	require.Len(t, snap.Resources, 0)
}

func TestErrorHooks_RetrySemanticsAndNoRetryWhenNoHooks_Create(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name          string
		withHooks     bool
		expectFailure bool
	}
	scenarios := []scenario{
		{name: "retry_if_any_hook_returns_true", withHooks: true, expectFailure: false},
		{name: "no_retry_when_no_hooks", withHooks: false, expectFailure: true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					createCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							createCalls++
							if createCalls == 1 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()

				var hooks []*deploytest.ResourceHook
				if s.withHooks {
					h, err := deploytest.NewHook(monitor, callbacks, "hook",
						func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
							_, _, _, _ resource.PropertyMap, _ string, _ []string,
						) (bool, error) {
							hookCalls++
							return true, nil
						}, true)
					require.NoError(t, err)
					hooks = []*deploytest.ResourceHook{h}
				}

				_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
					ResourceHookBindings: deploytest.ResourceHookBindings{
						OnError: hooks,
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
			if s.expectFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if s.withHooks {
				require.Equal(t, 1, hookCalls)
			} else {
				require.Equal(t, 0, hookCalls)
			}
		})
	}
}

func TestErrorHooks_RetrySemanticsAndNoRetryWhenNoHooks_Update(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name          string
		withHooks     bool
		expectFailure bool
	}
	scenarios := []scenario{
		{name: "retry_if_any_hook_returns_true", withHooks: true, expectFailure: false},
		{name: "no_retry_when_no_hooks", withHooks: false, expectFailure: true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					updateCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{ID: "id", Properties: req.Properties, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							updateCalls++
							if updateCalls == 1 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			isUpdate := false

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()

				var hooks []*deploytest.ResourceHook
				if s.withHooks {
					h, err := deploytest.NewHook(monitor, callbacks, "hook",
						func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
							_, _, _, _ resource.PropertyMap, _ string, _ []string,
						) (bool, error) {
							hookCalls++
							return true, nil
						}, true)
					require.NoError(t, err)
					hooks = []*deploytest.ResourceHook{h}
				}

				inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
				if isUpdate {
					inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
				}

				_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
					Inputs: inputs,
					ResourceHookBindings: deploytest.ResourceHookBindings{
						OnError: hooks,
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

			// Create
			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
			require.NoError(t, err)

			// Update
			isUpdate = true
			_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
			if s.expectFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if s.withHooks {
				require.Equal(t, 1, hookCalls)
			} else {
				require.Equal(t, 0, hookCalls)
			}
		})
	}
}

func TestErrorHooks_RetrySemanticsAndNoRetryWhenNoHooks_Delete(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name          string
		withHooks     bool
		expectFailure bool
	}
	scenarios := []scenario{
		{name: "retry_if_any_hook_returns_true", withHooks: true, expectFailure: false},
		{name: "no_retry_when_no_hooks", withHooks: false, expectFailure: true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					deleteCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							deleteCalls++
							if deleteCalls == 1 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			programCreate := true

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()

				var hooks []*deploytest.ResourceHook
				if s.withHooks {
					h, err := deploytest.NewHook(monitor, callbacks, "hook",
						func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
							_, _, _, _ resource.PropertyMap, _ string, _ []string,
						) (bool, error) {
							hookCalls++
							return true, nil
						}, true)
					require.NoError(t, err)
					hooks = []*deploytest.ResourceHook{h}
				}

				if programCreate {
					_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
						ResourceHookBindings: deploytest.ResourceHookBindings{
							OnError: hooks,
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

			// Create
			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
			require.NoError(t, err)

			// Delete
			programCreate = false
			_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
			if s.expectFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if s.withHooks {
				require.Equal(t, 1, hookCalls)
			} else {
				require.Equal(t, 0, hookCalls)
			}
		})
	}
}

func TestErrorHooks_NoRetryIfAllHooksReturnFalse_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					return plugin.CreateResponse{
						ID:     resource.ID("partial-id-" + req.URN.Name()),
						Status: resource.StatusPartialFailure,
					}, errors.New("create failed")
				},
			}, nil
		}),
	}

	hookCalls := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return false, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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
	require.Error(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_NoRetryIfAllHooksReturnFalse_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusOK}, nil
					}
					return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return false, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Update should fail (no retry because hook returns false)
	isUpdate = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.Error(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_NoRetryIfAllHooksReturnFalse_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
				},
			}, nil
		}),
	}

	hookCalls := 0
	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return false, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{h},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Delete should fail (no retry because hook returns false)
	programCreate = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.Error(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_NotCalledOnSuccess_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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
	require.NoError(t, err)

	require.Equal(t, 0, hookCalls)
}

func TestErrorHooks_NotCalledOnSuccess_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Update (successful; hook should not be called)
	isUpdate = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Equal(t, 0, hookCalls)
}

func TestErrorHooks_NotCalledOnSuccess_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{h},
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

	// Create
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Delete (successful; hook should not be called)
	programCreate = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Equal(t, 0, hookCalls)
}

func TestErrorHooks_RetryLimitWarningAt100_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					return plugin.CreateResponse{
						ID:     resource.ID("partial-id-" + req.URN.Name()),
						Status: resource.StatusPartialFailure,
					}, errors.New("create failed")
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				// Always retry until max retries is reached.
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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

	validateWarn := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.Error(t, err)
		require.ErrorContains(t, err, "maximum number of error hook retries reached")

		sawWarning := false
		for _, evt := range evts {
			if evt.Type != DiagEvent {
				continue
			}
			d := evt.Payload().(DiagEventPayload)
			if d.Severity == diag.Warning && strings.Contains(d.Message, "maximum number of error hook retries") {
				sawWarning = true
				break
			}
		}
		require.True(t, sawWarning, "expected a warning diagnostic when retry limit is hit")
		return err
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validateWarn, "0")
	require.Error(t, err)
}

func TestErrorHooks_RetryLimitWarningAt100_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusOK}, nil
					}
					return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
				},
			}, nil
		}),
	}

	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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
	validateCreate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, _ []Event, err error) error {
		require.NoError(t, err)
		isUpdate = true
		return nil
	}
	validateWarn := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.Error(t, err)
		require.ErrorContains(t, err, "maximum number of error hook retries reached")

		sawWarning := false
		for _, evt := range evts {
			if evt.Type != DiagEvent {
				continue
			}
			d := evt.Payload().(DiagEventPayload)
			if d.Severity == diag.Warning && strings.Contains(d.Message, "maximum number of error hook retries") {
				sawWarning = true
				break
			}
		}
		require.True(t, sawWarning, "expected a warning diagnostic when retry limit is hit")
		return err
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validateCreate, "0")
	require.NoError(t, err)
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validateWarn, "1")
	require.Error(t, err)
}

func TestErrorHooks_RetryLimitWarningAt100_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
				},
			}, nil
		}),
	}

	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				require.NotEmpty(t, errs)
				return true, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{h},
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
	validateCreate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, _ []Event, err error) error {
		require.NoError(t, err)
		programCreate = false
		return nil
	}
	validateWarn := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
		require.Error(t, err)
		require.ErrorContains(t, err, "maximum number of error hook retries reached")

		sawWarning := false
		for _, evt := range evts {
			if evt.Type != DiagEvent {
				continue
			}
			d := evt.Payload().(DiagEventPayload)
			if d.Severity == diag.Warning && strings.Contains(d.Message, "maximum number of error hook retries") {
				sawWarning = true
				break
			}
		}
		require.True(t, sawWarning, "expected a warning diagnostic when retry limit is hit")
		return err
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validateCreate, "0")
	require.NoError(t, err)
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validateWarn, "1")
	require.Error(t, err)
}

func TestErrorHooks_RetryOnceThenSuccess_HookCalledOnce_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			createCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					createCalls++
					if createCalls == 1 {
						return plugin.CreateResponse{
							ID:     resource.ID("partial-id-" + req.URN.Name()),
							Status: resource.StatusPartialFailure,
						}, errors.New("create failed")
					}
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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
	require.NoError(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_RetryOnceThenSuccess_HookCalledOnce_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			updateCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusOK}, nil
					}
					updateCalls++
					if updateCalls == 1 {
						return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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

	isUpdate = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_RetryOnceThenSuccess_HookCalledOnce_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			deleteCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					if deleteCalls == 1 {
						return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				hookCalls++
				return true, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{h},
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	programCreate = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Equal(t, 1, hookCalls)
}

func TestErrorHooks_RetryThenNoRetry_OperationFails_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			createCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					createCalls++
					if createCalls <= 2 {
						return plugin.CreateResponse{
							ID:     resource.ID("partial-id-" + req.URN.Name()),
							Status: resource.StatusPartialFailure,
						}, errors.New(fmt.Sprintf("create failed %d", createCalls))
					}
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				hookCalls++
				if hookCalls == 1 {
					require.Equal(t, []string{"create failed 1"}, errs)
				} else if hookCalls == 2 {
					require.Equal(t, []string{"create failed 2", "create failed 1"}, errs)
				}
				return hookCalls == 1, nil
			}, true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.NewPropertyMapFromMap(map[string]any{"v": "a"}),
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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
	require.Error(t, err)
	require.Equal(t, 2, hookCalls)
}

func TestErrorHooks_RetryThenNoRetry_OperationFails_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			updateCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusOK}, nil
					}
					updateCalls++
					if updateCalls <= 2 {
						return plugin.UpdateResponse{Status: resource.StatusPartialFailure},
							errors.New(fmt.Sprintf("update failed %d", updateCalls))
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				hookCalls++
				if hookCalls == 1 {
					require.Equal(t, []string{"update failed 1"}, errs)
				} else if hookCalls == 2 {
					require.Equal(t, []string{"update failed 2", "update failed 1"}, errs)
				}
				return hookCalls == 1, nil
			}, true)
		require.NoError(t, err)

		inputs := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputs = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputs,
			ResourceHookBindings: deploytest.ResourceHookBindings{
				OnError: []*deploytest.ResourceHook{h},
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

	isUpdate = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err) || err != nil)
	require.Equal(t, 2, hookCalls)
}

func TestErrorHooks_RetryThenNoRetry_OperationFails_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			deleteCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls++
					if deleteCalls <= 2 {
						return plugin.DeleteResponse{Status: resource.StatusPartialFailure},
							errors.New(fmt.Sprintf("delete failed %d", deleteCalls))
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	hookCalls := 0
	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		h, err := deploytest.NewHook(monitor, callbacks, "hook",
			func(_ context.Context, _ resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, errs []string,
			) (bool, error) {
				hookCalls++
				if hookCalls == 1 {
					require.Equal(t, []string{"delete failed 1"}, errs)
				} else if hookCalls == 2 {
					require.Equal(t, []string{"delete failed 2", "delete failed 1"}, errs)
				}
				return hookCalls == 1, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{
					OnError: []*deploytest.ResourceHook{h},
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	programCreate = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.True(t, result.IsBail(err) || err != nil)
	require.Equal(t, 2, hookCalls)
}

func TestErrorHooks_IndependentPerResource_Create(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			resACalls := 0
			resBCalls := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.Preview {
						return plugin.CreateResponse{Status: resource.StatusOK}, nil
					}
					switch req.URN.Name() {
					case "resA":
						resACalls++
						if resACalls == 1 {
							return plugin.CreateResponse{
								ID:     resource.ID("partial-id-" + req.URN.Name()),
								Status: resource.StatusPartialFailure,
							}, errors.New("resA create failed")
						}
					case "resB":
						resBCalls++
						if resBCalls <= 2 {
							return plugin.CreateResponse{
								ID:     resource.ID("partial-id-" + req.URN.Name()),
								Status: resource.StatusPartialFailure,
							}, errors.New("resB create failed")
						}
					}
					return plugin.CreateResponse{ID: resource.ID("id-" + req.URN.Name()), Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	resAHooks := 0
	resBHooks := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hA, err := deploytest.NewHook(monitor, callbacks, "hook-A",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resA" {
					resAHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		hB, err := deploytest.NewHook(monitor, callbacks, "hook-B",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resB" {
					resBHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		inputsA := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		inputsB := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:               inputsA,
			ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hA}},
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:               inputsB,
			ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hB}},
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
	require.NotNil(t, snap)

	require.Len(t, snap.Resources, 3) // default + 2 resources
	require.GreaterOrEqual(t, resAHooks, 1)
	require.GreaterOrEqual(t, resBHooks, 2)
}

func TestErrorHooks_IndependentPerResource_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			resAUpdates := 0
			resBUpdates := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: resource.ID("id-" + req.URN.Name()), Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					if req.Preview {
						return plugin.UpdateResponse{Status: resource.StatusOK}, nil
					}
					switch req.URN.Name() {
					case "resA":
						resAUpdates++
						if resAUpdates == 1 {
							return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("resA update failed")
						}
					case "resB":
						resBUpdates++
						if resBUpdates <= 2 {
							return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("resB update failed")
						}
					}
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	resAHooks := 0
	resBHooks := 0
	isUpdate := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hA, err := deploytest.NewHook(monitor, callbacks, "hook-A",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resA" {
					resAHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		hB, err := deploytest.NewHook(monitor, callbacks, "hook-B",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resB" {
					resBHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		inputsA := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		inputsB := resource.NewPropertyMapFromMap(map[string]any{"v": "a"})
		if isUpdate {
			inputsA = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
			inputsB = resource.NewPropertyMapFromMap(map[string]any{"v": "b"})
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:               inputsA,
			ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hA}},
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:               inputsB,
			ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hB}},
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
	isUpdate = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.GreaterOrEqual(t, resAHooks, 1)
	require.GreaterOrEqual(t, resBHooks, 2)
}

func TestErrorHooks_IndependentPerResource_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			resADeletes := 0
			resBDeletes := 0
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: resource.ID("id-" + req.URN.Name()), Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					switch req.URN.Name() {
					case "resA":
						resADeletes++
						if resADeletes == 1 {
							return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("resA delete failed")
						}
					case "resB":
						resBDeletes++
						if resBDeletes <= 2 {
							return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("resB delete failed")
						}
					}
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	resAHooks := 0
	resBHooks := 0
	programCreate := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		hA, err := deploytest.NewHook(monitor, callbacks, "hook-A",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resA" {
					resAHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		hB, err := deploytest.NewHook(monitor, callbacks, "hook-B",
			func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
				_, _, _, _ resource.PropertyMap, _ string, _ []string,
			) (bool, error) {
				if urn.Name() == "resB" {
					resBHooks++
				}
				return true, nil
			}, true)
		require.NoError(t, err)

		if programCreate {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hA}},
			})
			require.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				ResourceHookBindings: deploytest.ResourceHookBindings{OnError: []*deploytest.ResourceHook{hB}},
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

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	programCreate = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.GreaterOrEqual(t, resAHooks, 1)
	require.GreaterOrEqual(t, resBHooks, 2)
}
