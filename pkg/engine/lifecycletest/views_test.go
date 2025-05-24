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
	"fmt"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestViewsBasic(t *testing.T) {
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": req.Properties["foo"],
								},
								Outputs: resource.PropertyMap{
									"result": req.Properties["foo"],
								},
							},
						},
					})
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter++
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldInputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewStringProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewStringProperty("bar"),
							},
						},
					}, req.OldViews)

					// Update the view.
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.UpdateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpUpdate,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": req.NewInputs["foo"],
								},
								Outputs: resource.PropertyMap{
									"result": req.NewInputs["foo"],
								},
							},
						},
					})
					if err != nil {
						return plugin.UpdateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewStringProperty("baz"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewStringProperty("baz"),
							},
						},
					}, req.OldViews)

					// Delete the view.
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.DeleteResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					if err != nil {
						return plugin.DeleteResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	creating := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if creating {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSummaryEvent := func(expected display.ResourceChanges) lt.ValidateFunc {
		return func(_ workspace.Project, _ deploy.Target, _ engine.JournalEntries, events []engine.Event, _ error) error {
			var summaryEvent engine.Event
			for _, e := range events {
				if e.Type == engine.SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(engine.SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run a second update, should be same.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpSame: 2,
		}), "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run a third update, with a change, should update.
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate: 2,
		}), "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("baz"),
	}, snap.Resources[2].Outputs)

	// Run a fourth update, this time, deleting the resource and its view.
	creating = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpDelete: 2,
		}), "3")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}

func TestViewsRefresh(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("bar"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("bar"),
								},
							},
						},
					})
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	p.Steps = []lt.TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	// TODO test updating views from the refresh.
}

func TestViewsImport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("bar"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("bar"),
								},
							},
						},
					})
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
		},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	p.Steps = []lt.TestStep{{Op: lt.ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}})}}
	snap = p.Run(t, snap)
	assert.NotNil(t, snap)

	// TODO test creating new views from the Read for the imported resource.
}

func TestViewsDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": req.Properties["foo"],
								},
								Outputs: resource.PropertyMap{
									"result": req.Properties["foo"],
								},
							},
						},
					})
					if err != nil {
						return plugin.CreateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldInputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewStringProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewStringProperty("bar"),
							},
						},
					}, req.OldViews)

					// Update the view.
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					if err != nil {
						return plugin.UpdateResponse{}, fmt.Errorf("creating resource status client: %w", err)
					}
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     deploy.OpDeleteReplaced,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
						{
							Op:     deploy.OpReplace,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": req.NewInputs["foo"],
								},
								Outputs: resource.PropertyMap{
									"result": req.NewInputs["foo"],
								},
							},
						},
						{
							Op:     deploy.OpCreateReplacement,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": req.NewInputs["foo"],
								},
								Outputs: resource.PropertyMap{
									"result": req.NewInputs["foo"],
								},
							},
						},
					})
					if err != nil {
						return plugin.UpdateResponse{}, fmt.Errorf("publishing view steps: %w", err)
					}

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	creating := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if creating {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs: ins,
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSummaryEvent := func(expected display.ResourceChanges) lt.ValidateFunc {
		return func(_ workspace.Project, _ deploy.Target, _ engine.JournalEntries, events []engine.Event, _ error) error {
			var summaryEvent engine.Event
			for _, e := range events {
				if e.Type == engine.SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(engine.SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run another update, with a change, should update.
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate:  1,
			deploy.OpReplace: 1,
		}), "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("baz"),
	}, snap.Resources[2].Outputs)
}
