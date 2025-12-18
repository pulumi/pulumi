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
	"sync"
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

// These tests exercise the on-error hook + retry behavior end-to-end via the lifecycle test framework.

func TestErrorHooks_OperationIdentifierAndMultipleHooks(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					// Operation will fail exactly once (partial failure) then succeed; this ensures
					// on-error hooks run exactly once and see the correct failed operation identifier.
					var mu sync.Mutex
					createCalls := 0
					updateCalls := 0
					deleteCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							createCalls++
							if op == "create" && createCalls == 1 && !req.Preview {
								// Partial-failure creates must still return a stable ID so the engine can track/clean up
								// the partially-created resource.
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
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							updateCalls++
							if op == "update" && updateCalls == 1 && !req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure},
									errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							deleteCalls++
							if op == "delete" && deleteCalls == 1 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure},
									errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			var hook1Called, hook2Called bool
			var hook1Op, hook2Op string

			programCreate := true
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

				if programCreate {
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
				}

				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				step := lt.TestStep{Op: Update, SkipPreview: true}
				p.Steps = []lt.TestStep{step}
				snap := p.Run(t, nil)
				require.True(t, hook1Called)
				require.True(t, hook2Called)
				require.Equal(t, "create", hook1Op)
				require.Equal(t, "create", hook2Op)
				require.Len(t, snap.Resources, 2)
			case "update":
				// Create
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false,
					p.BackendClient, nil, "0")
				require.NoError(t, err)
				require.NotNil(t, snap)
				require.Len(t, snap.Resources, 2)
				require.False(t, hook1Called)
				require.False(t, hook2Called)

				// Update (fails once, should run on-error hooks with "update")
				isUpdate = true
				snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false,
					p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.NotNil(t, snap)
				require.True(t, hook1Called)
				require.True(t, hook2Called)
				require.Equal(t, "update", hook1Op)
				require.Equal(t, "update", hook2Op)
			case "delete":
				// Create
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false,
					p.BackendClient, nil, "0")
				require.NoError(t, err)
				require.NotNil(t, snap)
				require.Len(t, snap.Resources, 2)
				require.False(t, hook1Called)
				require.False(t, hook2Called)

				// Delete
				programCreate = false
				snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false,
					p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.True(t, hook1Called)
				require.True(t, hook2Called)
				require.Equal(t, "delete", hook1Op)
				require.Equal(t, "delete", hook2Op)
				require.Len(t, snap.Resources, 0)
			default:
				require.Fail(t, "unknown op")
			}
		})
	}
}

func TestErrorHooks_RetrySemanticsAndNoRetryWhenNoHooks(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					var mu sync.Mutex
					createCalls := 0
					updateCalls := 0
					deleteCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							createCalls++
							if op == "create" && createCalls == 1 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							updateCalls++
							if op == "update" && updateCalls == 1 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							deleteCalls++
							if op == "delete" && deleteCalls == 1 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			type scenario struct {
				name          string
				withHooks     bool
				expectFailure bool
				expectRetries int // additional attempts beyond the first for the failing op
			}

			scenarios := []scenario{
				{name: "retry_if_any_hook_returns_true", withHooks: true, expectFailure: false, expectRetries: 1},
				{name: "no_retry_when_no_hooks", withHooks: false, expectFailure: true, expectRetries: 0},
			}

			for _, s := range scenarios {
				t.Run(s.name, func(t *testing.T) {
					var mu sync.Mutex
					hookCalls := 0

					programCreate := true
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
									mu.Lock()
									defer mu.Unlock()
									hookCalls++
									return true, nil
								}, true)
							require.NoError(t, err)
							hooks = []*deploytest.ResourceHook{h}
						}

						if programCreate {
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
						}
						err = monitor.SignalAndWaitForShutdown(context.Background())
						require.NoError(t, err)
						return nil
					})

					hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
					p := &lt.TestPlan{
						Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
					}
					project := p.GetProject()

					switch op {
					case "create":
						p.Steps = []lt.TestStep{{
							Op:            Update,
							SkipPreview:   true,
							ExpectFailure: s.expectFailure,
						}}
						p.Run(t, nil)
						if s.withHooks {
							require.Equal(t, 1, hookCalls)
						} else {
							require.Equal(t, 0, hookCalls)
						}
					case "update":
						// Create
						snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false,
							p.BackendClient, nil, "0")
						require.NoError(t, err)
						// Update
						isUpdate = true
						_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false,
							p.BackendClient, nil, "1")
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
					case "delete":
						// Create
						snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false,
							p.BackendClient, nil, "0")
						require.NoError(t, err)
						// Delete
						programCreate = false
						_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false,
							p.BackendClient, nil, "1")
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
					}
				})
			}
		})
	}
}

func TestErrorHooks_NoRetryIfAllHooksReturnFalse(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							if op == "create" {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							if op == "update" {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							if op == "delete" {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			programCreate := true
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

				if programCreate {
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
				}

				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true, ExpectFailure: true}}
				p.Run(t, nil)
				require.Equal(t, 1, hookCalls)
			case "update":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				isUpdate = true
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.Error(t, err)
				require.Equal(t, 1, hookCalls)
			case "delete":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				programCreate = false
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.Error(t, err)
				require.Equal(t, 1, hookCalls)
			}
		})
	}
}

func TestErrorHooks_NotCalledOnSuccess(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
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
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			programCreate := true
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

				if programCreate {
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
				}
				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true}}
				p.Run(t, nil)
				require.Equal(t, 0, hookCalls)
			case "update":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				isUpdate = true
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.Equal(t, 0, hookCalls)
			case "delete":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				programCreate = false
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.Equal(t, 0, hookCalls)
			}
		})
	}
}

func TestErrorHooks_RetryLimitWarningAt100(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							if op == "create" {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							if op == "update" {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							if op == "delete" {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			programCreate := true
			isUpdate := false

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

				if programCreate {
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
				}
				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}

			validateWarn := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, evts []Event, err error) error {
				// Expect the retry limit error and a warning diagnostic about hitting the limit.
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

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{
					Op:            Update,
					SkipPreview:   true,
					ExpectFailure: true,
					Validate:      validateWarn,
				}}
				p.Run(t, nil)
			case "update":
				p.Steps = []lt.TestStep{
					{
						Op:          Update,
						SkipPreview: true,
						Validate: func(_ workspace.Project, _ deploy.Target, _ JournalEntries, _ []Event, err error) error {
							require.NoError(t, err)
							isUpdate = true
							return nil
						},
					},
					{
						Op:            Update,
						SkipPreview:   true,
						ExpectFailure: true,
						Validate:      validateWarn,
					},
				}
				p.Run(t, nil)
			case "delete":
				p.Steps = []lt.TestStep{
					{
						Op:          Update,
						SkipPreview: true,
						Validate: func(_ workspace.Project, _ deploy.Target, _ JournalEntries, _ []Event, err error) error {
							require.NoError(t, err)
							programCreate = false
							return nil
						},
					},
					{
						Op:            Update,
						SkipPreview:   true,
						ExpectFailure: true,
						Validate:      validateWarn,
					},
				}
				p.Run(t, nil)
			}
		})
	}
}

func TestErrorHooks_RetryOnceThenSuccess_HookCalledOnce(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					var mu sync.Mutex
					createCalls := 0
					updateCalls := 0
					deleteCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							createCalls++
							if op == "create" && createCalls == 1 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							updateCalls++
							if op == "update" && updateCalls == 1 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							deleteCalls++
							if op == "delete" && deleteCalls == 1 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			programCreate := true
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

				if programCreate {
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
				}
				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true}}
				p.Run(t, nil)
				require.Equal(t, 1, hookCalls)
			case "update":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				isUpdate = true
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.Equal(t, 1, hookCalls)
			case "delete":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				programCreate = false
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.Equal(t, 1, hookCalls)
			}
		})
	}
}

func TestErrorHooks_RetryThenNoRetry_OperationFails(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					var mu sync.Mutex
					createCalls := 0
					updateCalls := 0
					deleteCalls := 0

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							createCalls++
							if op == "create" && createCalls <= 2 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + req.URN.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("create failed")
							}
							return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							updateCalls++
							if op == "update" && updateCalls <= 2 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							deleteCalls++
							if op == "delete" && deleteCalls <= 2 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			hookCalls := 0
			programCreate := true
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
						// Retry once, then stop retrying.
						return hookCalls == 1, nil
					}, true)
				require.NoError(t, err)

				if programCreate {
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
				}
				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true, ExpectFailure: true}}
				p.Run(t, nil)
				require.Equal(t, 2, hookCalls)
			case "update":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				isUpdate = true
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.True(t, result.IsBail(err) || err != nil)
				require.Equal(t, 2, hookCalls)
			case "delete":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				programCreate = false
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.True(t, result.IsBail(err) || err != nil)
				require.Equal(t, 2, hookCalls)
			}
		})
	}
}

func TestErrorHooks_IndependentPerResource(t *testing.T) {
	t.Parallel()

	for _, op := range []string{"create", "update", "delete"} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					var mu sync.Mutex
					createCalls := map[resource.URN]int{}
					updateCalls := map[resource.URN]int{}
					deleteCalls := map[resource.URN]int{}

					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.CreateResponse{Status: resource.StatusOK}, nil
							}
							u := req.URN
							createCalls[u]++
							if op == "create" && u.Name() == "resA" && createCalls[u] == 1 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + u.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("resA create failed")
							}
							if op == "create" && u.Name() == "resB" && createCalls[u] <= 2 {
								return plugin.CreateResponse{
									ID:     resource.ID("partial-id-" + u.Name()),
									Status: resource.StatusPartialFailure,
								}, errors.New("resB create failed")
							}
							return plugin.CreateResponse{ID: resource.ID("id-" + u.Name()), Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
						},
						UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							if req.Preview {
								return plugin.UpdateResponse{Status: resource.StatusOK}, nil
							}
							u := req.URN
							updateCalls[u]++
							if op == "update" && u.Name() == "resA" && updateCalls[u] == 1 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("resA update failed")
							}
							if op == "update" && u.Name() == "resB" && updateCalls[u] <= 2 {
								return plugin.UpdateResponse{Status: resource.StatusPartialFailure}, errors.New("resB update failed")
							}
							return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
						},
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							mu.Lock()
							defer mu.Unlock()
							u := req.URN
							deleteCalls[u]++
							if op == "delete" && u.Name() == "resA" && deleteCalls[u] == 1 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("resA delete failed")
							}
							if op == "delete" && u.Name() == "resB" && deleteCalls[u] <= 2 {
								return plugin.DeleteResponse{Status: resource.StatusPartialFailure}, errors.New("resB delete failed")
							}
							return plugin.DeleteResponse{Status: resource.StatusOK}, nil
						},
					}, nil
				}),
			}

			var mu sync.Mutex
			hookCalls := map[string]int{"resA": 0, "resB": 0}

			programCreate := true
			isUpdate := false

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()

				mkHook := func(name string) *deploytest.ResourceHook {
					h, err := deploytest.NewHook(monitor, callbacks, "hook-"+name,
						func(_ context.Context, urn resource.URN, _ resource.ID, _ string, _ tokens.Type,
							_, _, _, _ resource.PropertyMap, _ string, _ []string,
						) (bool, error) {
							mu.Lock()
							defer mu.Unlock()
							hookCalls[urn.Name()]++
							return true, nil
						}, true)
					require.NoError(t, err)
					return h
				}
				hA := mkHook("A")
				hB := mkHook("B")

				if programCreate {
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
				}

				err = monitor.SignalAndWaitForShutdown(context.Background())
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}
			project := p.GetProject()

			switch op {
			case "create":
				p.Steps = []lt.TestStep{{Op: Update, SkipPreview: true}}
				snap := p.Run(t, nil)
				require.Len(t, snap.Resources, 3) // default + 2 resources
				require.GreaterOrEqual(t, hookCalls["resA"], 1)
				require.GreaterOrEqual(t, hookCalls["resB"], 2)
			case "update":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				isUpdate = true
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.GreaterOrEqual(t, hookCalls["resA"], 1)
				require.GreaterOrEqual(t, hookCalls["resB"], 2)
			case "delete":
				snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
				require.NoError(t, err)
				programCreate = false
				_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
				require.NoError(t, err)
				require.GreaterOrEqual(t, hookCalls["resA"], 1)
				require.GreaterOrEqual(t, hookCalls["resB"], 2)
			}
		})
	}
}
