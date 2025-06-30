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
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestViewsBasic tests the basic functionality of views in a resource lifecycle, doing an update to create
// the resources including a view, then running a second update expecting sames, then a third update that
// should update, and finally deleting the resource and its view.
func TestViewsBasic(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpUpdate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.DeleteResponse{
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
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
		return func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var summaryEvent Event
			for _, e := range events {
				if e.Type == SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run a second update, should be same.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpSame: 2,
		}), "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run a third update with a change, should update.
	ins = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate: 2,
		}), "2")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run a fourth update, this time deleting the resource and its view.
	creating = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpDelete: 2,
		}), "3")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 0)
}

// TestViewsUpdateError tests that an error from a view update step is properly propagated.
func TestViewsUpdateError(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpUpdate,
							Status: resource.StatusOK,
							Error:  "something went wrong",
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: ins,
		})
		if expectError {
			assert.Error(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
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
		return func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var summaryEvent Event
			for _, e := range events {
				if e.Type == SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resource.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run a second update, we should get an error for the view.
	expectError = true
	ins = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "baz",
	})
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "something went wrong")
}

// TestViewsUpdateDelete tests that a view can be deleted during an update operation.
func TestViewsUpdateDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
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
		return func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var summaryEvent Event
			for _, e := range events {
				if e.Type == SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run an update that will delete the view.
	ins = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate: 1,
			deploy.OpDelete: 1,
		}), "1")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
}

// TestViewRefreshSame tests that same view steps work from a refresh operation.
func TestViewsRefreshSame(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
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

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpSame,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
}

// TestViews_RefreshBeforeUpdate_Same tests that same view steps work from a refresh operation.
func TestViews_RefreshBeforeUpdate_Same(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:                  "new-id",
						Properties:          req.Properties,
						Status:              resource.StatusOK,
						RefreshBeforeUpdate: true,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpSame,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:                  req.ID,
							Inputs:              req.Inputs,
							Outputs:             req.State,
							RefreshBeforeUpdate: true,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)
}

// TestViewsRefreshUpdate tests that an update view step works from a refresh operation.
func TestViewsRefreshUpdate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
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

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpUpdate,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type: req.OldViews[0].Type,
								Name: req.OldViews[0].Name,
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("baz"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("baz"),
								},
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

// TestViews_RefreshBeforeUpdate_Update tests that an update view step works from a refresh operation that happens
// due to RefreshBeforeUpdate being set to true on the owning resource.
func TestViews_RefreshBeforeUpdate_Update(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:                  "new-id",
						Properties:          req.Properties,
						Status:              resource.StatusOK,
						RefreshBeforeUpdate: true,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpUpdate,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
							New: &deploytest.ViewStepState{
								Type: req.OldViews[0].Type,
								Name: req.OldViews[0].Name,
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("baz"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("baz"),
								},
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:                  req.ID,
							Inputs:              req.Inputs,
							Outputs:             req.State,
							RefreshBeforeUpdate: true,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("baz"),
	}, snap.Resources[2].Outputs)
}

// TestViewsRefreshDelete tests that a delete view step works from a refresh operation.
func TestViewsRefreshDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
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

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
}

// TestViews_RefreshBeforeUpdate_Delete tests that a delete view step works from a refresh operation that happens
// due to RefreshBeforeUpdate being set to true on the owning resource.
func TestViews_RefreshBeforeUpdate_Delete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:                  "new-id",
						Properties:          req.Properties,
						Status:              resource.StatusOK,
						RefreshBeforeUpdate: true,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:                  req.ID,
							Inputs:              req.Inputs,
							Outputs:             req.State,
							RefreshBeforeUpdate: true,
						},
						Status: resource.StatusOK,
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
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
}

// TestViewsImport is a basic sanity test of calling import on a stack that has views.
func TestViewsImport(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
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

	// Initial update.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 4)
	assert.Equal(t, "new-id", snap.Resources[2].ID.String())
	assert.Equal(t, snap.Resources[2].URN, snap.Resources[3].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[3].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[3].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[3].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[3].Outputs)

	// Import.
	p.Steps = []lt.TestStep{{Op: lt.ImportOp([]deploy.Import{{
		Type: "pkgA:m:typA",
		Name: "resB",
		ID:   "imported-id",
	}})}}
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 5)
	assert.Equal(t, "new-id", snap.Resources[2].ID.String())
	assert.Equal(t, snap.Resources[2].URN, snap.Resources[3].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[3].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[3].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewStringProperty("bar"),
	}, snap.Resources[3].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewStringProperty("bar"),
	}, snap.Resources[3].Outputs)
	assert.Equal(t, "imported-id", snap.Resources[4].ID.String())
}

// TestViewsDeleteBeforeReplace tests that a sequence of DeleteBeforeReplace view steps works as expected.
func TestViewsDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDeleteReplaced,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
						{
							Op:     apitype.OpReplace,
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
							Op:     apitype.OpCreateReplacement,
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
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
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
		return func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var summaryEvent Event
			for _, e := range events {
				if e.Type == SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
	ins = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate:  1,
			deploy.OpReplace: 1,
		}), "2")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

// TestViewsCreateBeforeReplace tests that a sequence of CreateBeforeReplace view steps works as expected.
func TestViewsCreateBeforeReplace(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreateReplacement,
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
							Op:     apitype.OpReplace,
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
							Op:     apitype.OpDeleteReplaced,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.NewInputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]any{
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
		return func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var summaryEvent Event
			for _, e := range events {
				if e.Type == SummaryEvent {
					summaryEvent = e
					break
				}
			}
			assert.NotNil(t, summaryEvent)
			payload := summaryEvent.Payload().(SummaryEventPayload)
			assert.Equal(t, expected, payload.ResourceChanges)
			return nil
		}
	}

	// Run an update to create the resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpCreate: 2,
		}), "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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
	ins = resource.NewPropertyMapFromMap(map[string]any{
		"foo": "baz",
	})
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		validateSummaryEvent(display.ResourceChanges{
			deploy.OpUpdate:  1,
			deploy.OpReplace: 1,
		}), "2")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

// TestViewsRefreshDriftDeleteCreate_UpdateRefresh verifies that during a `pulumi up --refresh` operation, a view can be
// deleted from Read and created from Update. In this scenario, drift has happened and the view resource is gone and
// should be recreated.
func TestViewsRefreshDriftDeleteCreate_UpdateRefresh(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("baz"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("baz"),
								},
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.OldOutputs,
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

	// Run an initial update to create the resources.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run `pulumi up --refresh`. In Read the view is deleted. In Update the view is recreated.
	p.Steps = []lt.TestStep{{Op: Update}}
	p.Options.Refresh = true
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("baz"),
	}, snap.Resources[2].Outputs)
}

// TestViewsRefreshDriftDeleteCreate_RefreshBeforeUpdate verifies that during a `pulumi up` operation, a view can be
// deleted from Read and created from Update when the view's owning resource is marked RefreshBeforeUpdate. In this
// scenario, drift has happened and the view resource is gone and should be recreated.
func TestViewsRefreshDriftDeleteCreate_RefreshBeforeUpdate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:                  "new-id",
						Properties:          req.Properties,
						Status:              resource.StatusOK,
						RefreshBeforeUpdate: true,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:                  req.ID,
							Inputs:              req.Inputs,
							Outputs:             req.State,
							RefreshBeforeUpdate: true,
						},
						Status: resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("baz"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("baz"),
								},
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties:          req.OldOutputs,
						Status:              resource.StatusOK,
						RefreshBeforeUpdate: true,
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

	// Run an initial update to create the resources.
	// The owning resource is marked RefreshBeforeUpdate.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.True(t, snap.Resources[1].RefreshBeforeUpdate)
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run `pulumi up`. The owner resource is marked RefreshBeforeUpdate, so it will be refreshed.
	// In Read the view is deleted. In Update the view is recreated.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.True(t, snap.Resources[1].RefreshBeforeUpdate)
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("baz"),
	}, snap.Resources[2].Outputs)
}

// TestViewsRefreshDriftDeleteCreate_RefreshProgram verifies that during a `pulumi up --refresh --run-program`
// operation, a view can be deleted from Read and created from Update. In this scenario, drift has happened
// and the view resource is gone and should be recreated.
func TestViewsRefreshDriftDeleteCreate_RefreshProgram(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

					return plugin.CreateResponse{
						ID:         "new-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					// Check that the old view is expected.
					assert.Equal(t, []plugin.View{
						{
							Type: tokens.Type("pkgA:m:typAView"),
							Name: req.URN.Name() + "-child",
							Inputs: resource.PropertyMap{
								"input": resource.NewProperty("bar"),
							},
							Outputs: resource.PropertyMap{
								"result": resource.NewProperty("bar"),
							},
						},
					}, req.OldViews)

					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpDelete,
							Status: resource.StatusOK,
							Old: &deploytest.ViewStepState{
								Type:    req.OldViews[0].Type,
								Name:    req.OldViews[0].Name,
								Inputs:  req.OldViews[0].Inputs,
								Outputs: req.OldViews[0].Outputs,
							},
						},
					})
					require.NoError(t, err)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
							Status: resource.StatusOK,
							New: &deploytest.ViewStepState{
								Type: tokens.Type("pkgA:m:typAView"),
								Name: req.URN.Name() + "-child",
								Inputs: resource.PropertyMap{
									"input": resource.NewProperty("baz"),
								},
								Outputs: resource.PropertyMap{
									"result": resource.NewProperty("baz"),
								},
							},
						},
					})
					require.NoError(t, err)

					return plugin.UpdateResponse{
						Properties: req.OldOutputs,
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

	// Run an initial update to create the resources.
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("bar"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("bar"),
	}, snap.Resources[2].Outputs)

	// Run `pulumi up --refresh --run-program`. The owner resource will be refreshed.
	// In Read the view is deleted. In Update the view is recreated.
	p.Steps = []lt.TestStep{{Op: Update}}
	p.Options.Refresh = true
	p.Options.RefreshProgram = true

	snap = p.Run(t, snap)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, "new-id", snap.Resources[1].ID.String())
	assert.Equal(t, snap.Resources[1].URN, snap.Resources[2].ViewOf)
	assert.Equal(t, "resA-child", snap.Resources[2].URN.Name())
	assert.Equal(t, tokens.Type("pkgA:m:typAView"), snap.Resources[2].URN.Type())
	assert.Equal(t, resource.PropertyMap{
		"input": resource.NewProperty("baz"),
	}, snap.Resources[2].Inputs)
	assert.Equal(t, resource.PropertyMap{
		"result": resource.NewProperty("baz"),
	}, snap.Resources[2].Outputs)
}

// TestViewsDestroyPreview ensures delete steps for views are synthesized during
// destroy previews.
func TestViewsDestroyPreview(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					rs, err := deploytest.NewResourceStatus(req.ResourceStatusAddress)
					require.NoError(t, err)
					defer rs.Close()

					err = rs.PublishViewSteps(req.ResourceStatusToken, []deploytest.ViewStep{
						{
							Op:     apitype.OpCreate,
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
					require.NoError(t, err)

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
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewProperty("bar"),
			},
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)
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

	// Run a destroy preview and ensure we got a delete event for the view.
	_, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, _ error) error {
			var viewDeletePreEventFound, summaryEventFound bool
			for _, e := range events {
				//nolint:exhaustive // We intentionally do not handle all event types here.
				switch e.Type {
				case ResourcePreEvent:
					payload := e.Payload().(ResourcePreEventPayload)
					if payload.Metadata.URN.Name() == "resA-child" {
						viewDeletePreEventFound = true
					}
				case SummaryEvent:
					summaryEventFound = true
					payload := e.Payload().(SummaryEventPayload)
					assert.Equal(t, display.ResourceChanges{
						deploy.OpDelete: 2,
					}, payload.ResourceChanges)
				}
			}
			assert.True(t, viewDeletePreEventFound, "view delete event found")
			assert.True(t, summaryEventFound, "summary event found")

			return nil
		}, "1")
	require.NoError(t, err)
}
