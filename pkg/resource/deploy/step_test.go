// Copyright 2016-2023, Pulumi Corporation.
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

package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		op   display.StepOp
		want string
	}{
		{name: "Same", op: OpSame, want: "  "},
		{name: "Create", op: OpCreate, want: "+ "},
		{name: "Delete", op: OpDelete, want: "- "},
		{name: "Update", op: OpUpdate, want: "~ "},
		{name: "Replace", op: OpReplace, want: "+-"},
		{name: "CreateReplacement", op: OpCreateReplacement, want: "++"},
		{name: "DeleteReplaced", op: OpDeleteReplaced, want: "--"},
		{name: "Read", op: OpRead, want: "> "},
		{name: "ReadReplacement", op: OpReadReplacement, want: ">>"},
		{name: "Refresh", op: OpRefresh, want: "~ "},
		{name: "ReadDiscard", op: OpReadDiscard, want: "< "},
		{name: "DiscardReplaced", op: OpDiscardReplaced, want: "<<"},
		{name: "Import", op: OpImport, want: "= "},
		{name: "ImportReplacement", op: OpImportReplacement, want: "=>"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RawPrefix(tt.op))
		})
	}
	t.Run("panics on unknown", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			RawPrefix("not-a-real-operation")
		})
	})
}

func TestPastTense(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		op   display.StepOp
		want string
	}{
		{"Same", OpSame, "samed"},
		{"Create", OpCreate, "created"},
		{"Replace", OpReplace, "replaced"},
		{"Update", OpUpdate, "updated"},

		// TODO(dixler) consider fixing this.
		{"CreateReplacement", OpCreateReplacement, "create-replacementd"},
		{"ReadReplacement", OpReadReplacement, "read-replacementd"},

		{"Refresh", OpRefresh, "refreshed"},
		{"Read", OpRead, "read"},
		{"ReadDiscard", OpReadDiscard, "discarded"},
		{"DiscardReplaced", OpDiscardReplaced, "discarded"},
		{"Delete", OpDelete, "deleted"},
		{"DeleteReplaced", OpDeleteReplaced, "deleted"},
		{"Import", OpImport, "imported"},
		{"ImportReplacement", OpImportReplacement, "imported"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PastTense(tt.op))
		})
	}
	t.Run("panics on unknown", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			PastTense("not-a-real-operation")
		})
	})
}

func TestSameStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("bad provider state for resource", func(t *testing.T) {
			t.Parallel()
			s := &SameStep{
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				old: &resource.State{
					URN: "urn:pulumi:stack::project::type::foo",
				},
				new: &resource.State{
					URN:  "urn:pulumi:stack::project::type::foo",
					Type: "pulumi:providers:some-provider",
				},
			}
			_, _, err := s.Apply()
			assert.ErrorContains(t, err, "bad provider state for resource")
		})
	})
}

func TestCreateStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("custom", func(t *testing.T) {
			t.Parallel()
			t.Run("error getting provider", func(t *testing.T) {
				t.Parallel()
				s := &CreateStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
					},
					new: &resource.State{
						URN:    "urn:pulumi:stack::project::some-type::some-urn",
						Custom: true,
						// Use denydefaultprovider ID to ensure failure.
						Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "Default provider for 'default_5_42_0' disabled.")
				assert.Equal(t, resource.StatusOK, status)
			})
			t.Run("error in create", func(t *testing.T) {
				t.Parallel()
				expectedErr := errors.New("expected error")
				var createCalled bool
				s := &CreateStep{
					new: &resource.State{
						URN:    "urn:pulumi:stack::project::some-type::some-urn",
						Custom: true,
					},
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
					},
					provider: &deploytest.Provider{
						CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
							createCalled = true
							return plugin.CreateResponse{}, expectedErr
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorIs(t, err, expectedErr)
				assert.True(t, createCalled)
				assert.Equal(t, resource.StatusOK, status)
			})
			t.Run("handle InitError", func(t *testing.T) {
				t.Parallel()
				s := &CreateStep{
					new: &resource.State{
						URN:    "urn:pulumi:stack::project::some-type::some-urn",
						Custom: true,
					},
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
					},
					provider: &deploytest.Provider{
						CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{
									Status: resource.StatusPartialFailure,
								}, &plugin.InitError{
									Reasons: []string{
										"intentional error",
									},
								}
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "intentional error")
				require.Len(t, s.new.InitErrors, 1)
				assert.Equal(t, resource.StatusPartialFailure, status)
			})
			t.Run("error create no ID", func(t *testing.T) {
				t.Parallel()
				s := &CreateStep{
					new: &resource.State{
						URN:    "urn:pulumi:stack::project::some-type::some-urn",
						Custom: true,
					},
					deployment: &Deployment{
						opts: &Options{},
					},
					provider: &deploytest.Provider{
						CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{}, nil
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "provider did not return an ID from Create")
				assert.Equal(t, resource.StatusOK, status)
			})
		})
	})
}

func TestDeleteStep(t *testing.T) {
	t.Parallel()
	t.Run("isDeletedWith", func(t *testing.T) {
		t.Parallel()
		otherDeletions := map[resource.URN]bool{
			"false-key": false,
			"true-key":  true,
		}
		assert.False(t, isDeletedWith("", otherDeletions))
		assert.False(t, isDeletedWith("does-not-exist", otherDeletions))
		assert.False(t, isDeletedWith("false-key", otherDeletions))
		assert.True(t, isDeletedWith("true-key", otherDeletions))
	})
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("custom", func(t *testing.T) {
			t.Parallel()
			t.Run("error getting provider", func(t *testing.T) {
				t.Parallel()
				s := &DeleteStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: false,
						},
					},
					old: &resource.State{
						URN:    "urn:pulumi:stack::project::some-type::some-urn",
						Custom: true,
						// Use denydefaultprovider ID to ensure failure.
						Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "Default provider for 'default_5_42_0' disabled.")
				assert.Equal(t, resource.StatusOK, status)
			})
		})
	})
}

func TestRemovePendingReplaceStep(t *testing.T) {
	t.Parallel()
	t.Run("NewRemovePendingReplaceStep", func(t *testing.T) {
		t.Parallel()
		t.Run("panics on old=nil", func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				NewRemovePendingReplaceStep(nil, nil)
			})
		})
		t.Run("panics if not old.PendingReplacement", func(t *testing.T) {
			t.Parallel()
			assert.Panics(t, func() {
				NewRemovePendingReplaceStep(nil, &resource.State{
					PendingReplacement: false,
				})
			})
		})
	})
	t.Run("Op", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			PendingReplacement: true,
		})
		assert.Equal(t, OpRemovePendingReplace, s.Op())
	})
	t.Run("Deployment", func(t *testing.T) {
		t.Parallel()
		d := &Deployment{}
		s := NewRemovePendingReplaceStep(d, &resource.State{
			Type:               "expected-value",
			PendingReplacement: true,
		})
		assert.Equal(t, d, s.Deployment())
	})
	t.Run("Type", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			Type:               "expected-value",
			PendingReplacement: true,
		})
		assert.Equal(t, tokens.Type("expected-value"), s.Type())
	})
	t.Run("Provider", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			Provider:           "expected-value",
			PendingReplacement: true,
		})
		assert.Equal(t, "expected-value", s.Provider())
	})
	t.Run("URN", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			URN:                "expected-value",
			PendingReplacement: true,
		})
		assert.Equal(t, resource.URN("expected-value"), s.URN())
	})
	t.Run("Old", func(t *testing.T) {
		t.Parallel()
		old := &resource.State{
			URN:                "expected-value",
			PendingReplacement: true,
		}
		s := NewRemovePendingReplaceStep(nil, old)
		assert.Equal(t, old, s.Old())
	})
	t.Run("New", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			PendingReplacement: true,
		})
		assert.Equal(t, (*resource.State)(nil), s.New())
	})
	t.Run("Res", func(t *testing.T) {
		t.Parallel()
		old := &resource.State{
			PendingReplacement: true,
		}
		s := NewRemovePendingReplaceStep(nil, old)
		assert.Equal(t, old, s.Res())
	})
	t.Run("Logical", func(t *testing.T) {
		t.Parallel()
		s := NewRemovePendingReplaceStep(nil, &resource.State{
			PendingReplacement: true,
		})
		assert.False(t, s.Logical())
	})
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		d := &Deployment{
			opts: &Options{
				DryRun: true,
			},
		}

		s := NewRemovePendingReplaceStep(d, &resource.State{
			PendingReplacement: true,
		})
		status, _, err := s.Apply()
		require.NoError(t, err)
		assert.Equal(t, resource.StatusOK, status)
	})
}

func TestUpdateStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("error getting provider", func(t *testing.T) {
			t.Parallel()
			s := &UpdateStep{
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				old: &resource.State{},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::some-type::some-urn",
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "Default provider for 'default_5_42_0' disabled.")
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("failure in provider", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			s := &UpdateStep{
				old: &resource.State{},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::some-type::some-urn",
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				provider: &deploytest.Provider{
					UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
						return plugin.UpdateResponse{}, expectedErr
					},
				},
			}
			status, _, err := s.Apply()
			assert.ErrorIs(t, err, expectedErr)
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("partial failure in provider", func(t *testing.T) {
			t.Parallel()
			s := &UpdateStep{
				old: &resource.State{},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::some-type::some-urn",
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				provider: &deploytest.Provider{
					UpdateF: func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
						return plugin.UpdateResponse{
								Properties: resource.PropertyMap{
									"key": resource.NewStringProperty("expected-value"),
								},
								Status: resource.StatusPartialFailure,
							}, &plugin.InitError{
								Reasons: []string{
									"intentional error",
								},
							}
					},
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "intentional error")
			assert.Equal(t, resource.StatusPartialFailure, status)

			// News should be updated.
			require.Len(t, s.new.InitErrors, 1)
			assert.Equal(t, resource.PropertyMap{
				"key": resource.NewStringProperty("expected-value"),
			}, s.new.Outputs)
		})
	})
}

func TestReplaceStep(t *testing.T) {
	t.Parallel()
	t.Run("Deployment", func(t *testing.T) {
		t.Parallel()
		d := &Deployment{}
		s := ReplaceStep{deployment: d}
		assert.Equal(t, d, s.Deployment())
	})
}

func TestReadStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("error getting provider", func(t *testing.T) {
			t.Parallel()
			s := &ReadStep{
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				old: &resource.State{},
				new: &resource.State{
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "Default provider for 'default_5_42_0' disabled.")
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("failure in provider", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			s := &ReadStep{
				old: &resource.State{},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::some-type::some-urn",
					ID:     "some-id",
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				provider: &deploytest.Provider{
					ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{}, expectedErr
					},
				},
			}
			status, _, err := s.Apply()
			assert.ErrorIs(t, err, expectedErr)
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("partial failure in provider", func(t *testing.T) {
			t.Parallel()
			s := &ReadStep{
				old: &resource.State{},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::some-type::some-urn",
					ID:     "some-id",
					Custom: true,
					// Use denydefaultprovider ID to ensure failure.
					Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
				},
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				provider: &deploytest.Provider{
					ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
								ReadResult: plugin.ReadResult{
									ID: "new-id",
									Inputs: resource.PropertyMap{
										"inputs-key": resource.NewStringProperty("expected-value"),
									},
									Outputs: resource.PropertyMap{
										"outputs-key": resource.NewStringProperty("expected-value"),
									},
								},
								Status: resource.StatusPartialFailure,
							}, &plugin.InitError{
								Reasons: []string{
									"intentional error",
								},
							}
					},
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "intentional error")
			assert.Equal(t, resource.StatusPartialFailure, status)

			// News should be updated.
			require.Len(t, s.new.InitErrors, 1)
			assert.Equal(t, (resource.PropertyMap)(nil), s.new.Inputs)
			assert.Equal(t, resource.PropertyMap{
				"outputs-key": resource.NewStringProperty("expected-value"),
			}, s.new.Outputs)
			assert.Equal(t, resource.ID("new-id"), s.new.ID)
		})
		t.Run("unknown id", func(t *testing.T) {
			t.Parallel()
			s := &ReadStep{
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
				},
				new: &resource.State{
					ID: plugin.UnknownStringValue,
				},
				provider: &deploytest.Provider{
					ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
						panic("should not be called")
					},
				},
			}
			status, _, err := s.Apply()
			require.NoError(t, err)
			assert.Equal(t, resource.StatusOK, status)
			// News should be updated.
			assert.Equal(t, resource.PropertyMap{}, s.new.Outputs)
		})
	})
}

func TestRefreshStepPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		inputs               resource.PropertyMap
		outputs              resource.PropertyMap
		readInputs           resource.PropertyMap
		readOutputs          resource.PropertyMap
		diffResult           plugin.DiffResult
		expectedDetailedDiff map[string]plugin.PropertyDiff
		ignoreChanges        []string
	}{
		{
			name:   "tfbridge 'computed' property changed",
			inputs: resource.PropertyMap{},
			outputs: resource.PropertyMap{
				"etag": resource.NewStringProperty("abc"),
			},
			readInputs: resource.PropertyMap{},
			readOutputs: resource.PropertyMap{
				"etag": resource.NewStringProperty("def"),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes:      plugin.DiffNone,
				DetailedDiff: map[string]plugin.PropertyDiff{},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{},
		},
		{
			// Note: this is probably a case where the TF provider has a bug, a pure in property
			// really shouldn't change, but this is common in TF providers.
			name: "tfbridge 'required' property changed",
			inputs: resource.PropertyMap{
				"title": resource.NewStringProperty("test"),
			},
			outputs: resource.PropertyMap{
				"title": resource.NewStringProperty("test"),
			},
			readInputs: resource.PropertyMap{
				"title": resource.NewStringProperty("testtesttest"),
			},
			readOutputs: resource.PropertyMap{
				"title": resource.NewStringProperty("testtesttest"),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes: plugin.DiffSome,
				DetailedDiff: map[string]plugin.PropertyDiff{
					"title": {Kind: plugin.DiffUpdate},
				},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{
				"title": {Kind: plugin.DiffUpdate},
			},
		},
		{
			// Note: this is probably a case where the TF provider has a bug, a pure in property
			// really shouldn't change, but this is common in TF providers.
			name:          "tfbridge 'required' property changed w/ ignoreChanges",
			ignoreChanges: []string{"title"},
			inputs: resource.PropertyMap{
				"title": resource.NewStringProperty("test"),
			},
			outputs: resource.PropertyMap{
				"title": resource.NewStringProperty("test"),
			},
			readInputs: resource.PropertyMap{
				"title": resource.NewStringProperty("testtesttest"),
			},
			readOutputs: resource.PropertyMap{
				"title": resource.NewStringProperty("testtesttest"),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes:      plugin.DiffNone,
				DetailedDiff: map[string]plugin.PropertyDiff{},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{},
		},
		{
			// Note: this is probably a case where the TF provider has a bug, a pure in property
			// really shouldn't change, but this is common in TF providers.
			name:   "tfbridge 'optional' property changed",
			inputs: resource.PropertyMap{},
			outputs: resource.PropertyMap{
				"body": resource.NewStringProperty(""),
			},
			readInputs: resource.PropertyMap{
				// Pretty sure its a bug in tfbridge that it doesn't populate the new value
				// into inputs for an `optional` property.  But that's what it does so
				// testing against the current behaviour.
			},
			readOutputs: resource.PropertyMap{
				"body": resource.NewStringProperty("bodybodybody"),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes: plugin.DiffSome,
				DetailedDiff: map[string]plugin.PropertyDiff{
					"body": {Kind: plugin.DiffDelete},
				},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{
				"body": {Kind: plugin.DiffAdd},
			},
		},
		{
			// Note: this is probably a case where the TF provider has a bug, a pure in property
			// really shouldn't change, but this is common in TF providers.
			name:          "tfbridge 'optional' property changed w/ ignoreChanges",
			ignoreChanges: []string{"body"},
			inputs:        resource.PropertyMap{},
			outputs: resource.PropertyMap{
				"body": resource.NewStringProperty(""),
			},
			readInputs: resource.PropertyMap{
				// Pretty sure its a bug in tfbridge that it doesn't populate the new value
				// into inputs for an `optional` property.  But that's what it does so
				// testing against the current behaviour.
			},
			readOutputs: resource.PropertyMap{
				"body": resource.NewStringProperty("bodybodybody"),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes:      plugin.DiffNone,
				DetailedDiff: map[string]plugin.PropertyDiff{},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{},
		},
		{
			name:   "tfbridge 'optional+computed' property element added",
			inputs: resource.PropertyMap{},
			outputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty(resource.PropertyMap{}),
			},
			readInputs: resource.PropertyMap{},
			readOutputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty((resource.PropertyMap{
					"foo": resource.NewStringProperty("bar"),
				})),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes: plugin.DiffSome,
				DetailedDiff: map[string]plugin.PropertyDiff{
					"tags":     {Kind: plugin.DiffUpdate},
					"tags.foo": {Kind: plugin.DiffDelete},
				},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{
				"tags":     {Kind: plugin.DiffUpdate},
				"tags.foo": {Kind: plugin.DiffAdd},
			},
		},
		{
			name: "tfbridge 'optional+computed' property element changed",
			inputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty(resource.PropertyMap{
					"foo": resource.NewStringProperty("bar"),
				}),
			},
			outputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty(resource.PropertyMap{
					"foo": resource.NewStringProperty("bar"),
				}),
			},
			readInputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty((resource.PropertyMap{
					"foo": resource.NewStringProperty("baz"),
				})),
			},
			readOutputs: resource.PropertyMap{
				"tags": resource.NewObjectProperty((resource.PropertyMap{
					"foo": resource.NewStringProperty("baz"),
				})),
			},
			diffResult: plugin.DiffResult{
				// Diff newInputs, newOutputs, oldInputs
				Changes: plugin.DiffSome,
				DetailedDiff: map[string]plugin.PropertyDiff{
					"tags.foo": {Kind: plugin.DiffUpdate},
				},
			},
			expectedDetailedDiff: map[string]plugin.PropertyDiff{
				"tags.foo": {Kind: plugin.DiffUpdate},
			},
		},
	}

	for _, tc := range tests {
		state := &resource.State{
			URN:      "urn:pulumi:stack::project::some-type::some-urn",
			ID:       "some-id",
			Type:     "some-type",
			Custom:   true,
			Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
			Inputs:   tc.inputs,
			Outputs:  tc.outputs,
		}
		s := &RefreshStep{
			old: state,
			new: state.Copy(),
			deployment: &Deployment{
				opts: &Options{
					DryRun: true,
				},
			},
			provider: &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  tc.readInputs,
							Outputs: tc.readOutputs,
						},
						Status: resource.StatusOK,
					}, nil
				},
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
					return tc.diffResult, nil
				},
			},
		}
		status, _, err := s.Apply()
		assert.Equal(t, s.diff.DetailedDiff, tc.expectedDetailedDiff)
		require.NoError(t, err)
		assert.Equal(t, resource.StatusOK, status)
	}
}

func TestRefreshStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("error getting provider", func(t *testing.T) {
			t.Parallel()
			state := &resource.State{
				Custom: true,
				// Use denydefaultprovider ID to ensure failure.
				Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
			}
			s := NewRefreshStep(&Deployment{
				opts: &Options{
					DryRun: true,
				},
			}, nil, state, nil, nil)
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "Default provider for 'default_5_42_0' disabled.")
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("failure in provider", func(t *testing.T) {
			t.Parallel()
			expectedErr := errors.New("expected error")
			state := &resource.State{
				URN:    "urn:pulumi:stack::project::some-type::some-urn",
				ID:     "some-id",
				Custom: true,
				// Use denydefaultprovider ID to ensure failure.
				Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
			}
			s := NewRefreshStep(
				&Deployment{
					opts: &Options{
						DryRun: true,
					},
				}, nil, state, nil, nil).(*RefreshStep)
			s.provider = &deploytest.Provider{
				ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{}, expectedErr
				},
			}

			status, _, err := s.Apply()
			assert.ErrorIs(t, err, expectedErr)
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("partial failure in provider", func(t *testing.T) {
			t.Parallel()
			state := &resource.State{
				URN:    "urn:pulumi:stack::project::some-type::some-urn",
				ID:     "some-id",
				Type:   "some-type",
				Custom: true,
				// Use denydefaultprovider ID to ensure failure.
				Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default_5_42_0::denydefaultprovider",
			}
			s := NewRefreshStep(&Deployment{
				ctx: &plugin.Context{Diag: &deploytest.NoopSink{}},
				opts: &Options{
					DryRun: true,
				},
			}, nil, state, nil, nil).(*RefreshStep)
			s.provider = &deploytest.Provider{
				ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								ID: "new-id",
								Inputs: resource.PropertyMap{
									"inputs-key": resource.NewStringProperty("expected-value"),
								},
								Outputs: resource.PropertyMap{
									"outputs-key": resource.NewStringProperty("expected-value"),
								},
							},
							Status: resource.StatusPartialFailure,
						}, &plugin.InitError{
							Reasons: []string{
								"intentional error",
							},
						}
				},
			}
			status, _, err := s.Apply()
			require.NoError(t, err, "InitError should be discarded")
			assert.Equal(t, resource.StatusPartialFailure, status)

			// News should be updated.
			require.Len(t, s.new.InitErrors, 1)
			assert.Equal(t, resource.PropertyMap{
				"outputs-key": resource.NewStringProperty("expected-value"),
			}, s.new.Outputs)
			assert.Equal(t, resource.ID("new-id"), s.new.ID)
		})
	})
}

func TestImportStep(t *testing.T) {
	t.Parallel()
	t.Run("Apply", func(t *testing.T) {
		t.Parallel()
		t.Run("missing parent", func(t *testing.T) {
			t.Parallel()
			s := &ImportStep{
				planned: true,
				new: &resource.State{
					Parent: "urn:pulumi:stack::project::foo:bar:Bar::name",
				},
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
					olds: map[resource.URN]*resource.State{},
					news: &gsync.Map[urn.URN, *resource.State]{},
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "unknown parent")
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("getProvider error", func(t *testing.T) {
			t.Parallel()
			s := &ImportStep{
				deployment: &Deployment{
					opts: &Options{
						DryRun: true,
					},
					olds: map[resource.URN]*resource.State{},
					news: &gsync.Map[urn.URN, *resource.State]{},
				},
				new: &resource.State{
					URN:    "urn:pulumi:stack::project::foo:bar:Bar::name",
					ID:     "some-id",
					Custom: true,
				},
			}
			status, _, err := s.Apply()
			assert.ErrorContains(t, err, "bad provider reference")
			assert.Equal(t, resource.StatusOK, status)
		})
		t.Run("provider read error", func(t *testing.T) {
			t.Parallel()
			t.Run("error", func(t *testing.T) {
				t.Parallel()
				expectedErr := errors.New("expected error")
				s := &ImportStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
						olds: map[resource.URN]*resource.State{},
						news: &gsync.Map[urn.URN, *resource.State]{},
					},
					new: &resource.State{
						URN:      "urn:pulumi:stack::project::foo:bar:Bar::name",
						ImportID: "some-id",
						Custom:   true,
					},
					provider: &deploytest.Provider{
						ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
							return plugin.ReadResponse{}, expectedErr
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorIs(t, err, expectedErr)
				assert.Equal(t, resource.StatusOK, status)
			})
			t.Run("init error", func(t *testing.T) {
				t.Parallel()
				s := &ImportStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
						olds: map[resource.URN]*resource.State{},
						news: &gsync.Map[urn.URN, *resource.State]{},
					},
					new: &resource.State{
						URN:      "urn:pulumi:stack::project::foo:bar:Bar::name",
						ImportID: "some-id",
						Custom:   true,
					},
					provider: &deploytest.Provider{
						ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
							return plugin.ReadResponse{}, &plugin.InitError{
								Reasons: []string{
									"intentional error",
								},
							}
						},
					},
				}
				status, _, err := s.Apply()
				assert.Error(t, err)
				assert.Equal(t, resource.StatusOK, status)
				require.Len(t, s.new.InitErrors, 1)
			})
			t.Run("resource does not exist", func(t *testing.T) {
				t.Parallel()
				s := &ImportStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
						olds: map[resource.URN]*resource.State{},
						news: &gsync.Map[urn.URN, *resource.State]{},
					},
					new: &resource.State{
						URN:      "urn:pulumi:stack::project::foo:bar:Bar::name",
						ImportID: "some-id",
						Custom:   true,
					},
					provider: &deploytest.Provider{
						ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
							return plugin.ReadResponse{}, nil
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "does not exist")
				assert.Equal(t, resource.StatusOK, status)
			})
			t.Run("provider does not support importing resources", func(t *testing.T) {
				t.Parallel()
				s := &ImportStep{
					deployment: &Deployment{
						opts: &Options{
							DryRun: true,
						},
						olds: map[resource.URN]*resource.State{},
						news: &gsync.Map[urn.URN, *resource.State]{},
					},
					new: &resource.State{
						URN:      "urn:pulumi:stack::project::foo:bar:Bar::name",
						ImportID: "some-id",
						Custom:   true,
					},
					provider: &deploytest.Provider{
						ReadF: func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
							return plugin.ReadResponse{
								ReadResult: plugin.ReadResult{
									Outputs: resource.PropertyMap{},
								},
								Status: resource.StatusOK,
							}, nil
						},
					},
				}
				status, _, err := s.Apply()
				assert.ErrorContains(t, err, "provider does not support importing resources")
				assert.Equal(t, resource.StatusOK, status)
			})
		})
	})
}

func TestGetProvider(t *testing.T) {
	t.Parallel()
	t.Run("ensure default is not selected", func(t *testing.T) {
		t.Parallel()
		s := &CreateStep{
			new: &resource.State{
				Provider: "invalid-provider",
			},
		}
		prov, err := getProvider(s, s.provider)
		assert.Nil(t, prov)
		assert.ErrorContains(t, err, "bad provider reference")
	})
	t.Run("ensure default is not selected", func(t *testing.T) {
		t.Parallel()
		expectedProvider := &deploytest.Provider{}
		s := &CreateStep{
			provider: expectedProvider,
			new: &resource.State{
				Provider: "invalid-provider",
			},
		}
		prov, err := getProvider(s, s.provider)
		require.NoError(t, err)
		assert.Equal(t, expectedProvider, prov)
	})
}

func TestSuffix(t *testing.T) {
	t.Parallel()
	for op, expectation := range map[display.StepOp]string{
		OpSame:                 "",
		OpCreate:               "",
		OpDelete:               "",
		OpDeleteReplaced:       "",
		OpRead:                 "",
		OpReadDiscard:          "",
		OpDiscardReplaced:      "",
		OpRemovePendingReplace: "",
		OpImport:               "",
		"not-a-real-step-op":   "",

		OpCreateReplacement: colors.Reset,
		OpUpdate:            colors.Reset,
		OpReplace:           colors.Reset,
		OpReadReplacement:   colors.Reset,
		OpRefresh:           colors.Reset,
		OpImportReplacement: colors.Reset,
	} {
		assert.Equal(t, expectation, Suffix(op))
	}
}
