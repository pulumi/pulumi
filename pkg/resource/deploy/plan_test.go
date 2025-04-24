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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestPlan(t *testing.T) {
	t.Parallel()
	t.Run("ContainsDelete", func(t *testing.T) {
		t.Parallel()
		t.Run("does", func(t *testing.T) {
			t.Parallel()
			p := &PlanDiff{
				Deletes: []resource.PropertyKey{
					resource.PropertyKey("foo"),
				},
			}
			assert.True(t, p.ContainsDelete(resource.PropertyKey("foo")))
		})
		t.Run("doesn't (empty)", func(t *testing.T) {
			t.Parallel()
			p := &PlanDiff{}
			assert.False(t, p.ContainsDelete(resource.PropertyKey("foo")))
		})
		t.Run("doesn't", func(t *testing.T) {
			t.Parallel()
			p := &PlanDiff{
				Deletes: []resource.PropertyKey{
					resource.PropertyKey("not-foo"),
				},
			}
			assert.False(t, p.ContainsDelete(resource.PropertyKey("foo")))
		})
	})
	t.Run("MakeError", func(t *testing.T) {
		t.Parallel()
		t.Run("delete", func(t *testing.T) {
			t.Parallel()
			p := &PlanDiff{
				Deletes: []resource.PropertyKey{
					resource.PropertyKey("foo"),
				},
			}
			val := resource.NewStringProperty("val")
			errStr := p.MakeError(resource.PropertyKey("foo"), "", &val)
			assert.True(t, strings.HasPrefix(errStr, "-"))
		})
	})
}

func TestGoalPlan(t *testing.T) {
	t.Parallel()
	t.Run("NewGoalPlan", func(t *testing.T) {
		t.Parallel()
		t.Run("nil goal", func(t *testing.T) {
			t.Parallel()
			assert.Nil(t, NewGoalPlan(nil, nil))
		})
	})
}

func TestResourcePlan(t *testing.T) {
	t.Parallel()

	t.Run("diffPropertyKeys", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name          string
			keysOld       []resource.PropertyKey
			keysNew       []resource.PropertyKey
			expectResult  string
			expectChanged bool
		}{
			{
				name: "mutually exclusive keys",
				keysOld: []resource.PropertyKey{
					resource.PropertyKey("foo"),
					resource.PropertyKey("bar"),
					resource.PropertyKey("baz"),
				},
				keysNew: []resource.PropertyKey{
					resource.PropertyKey("a"),
					resource.PropertyKey("b"),
					resource.PropertyKey("c"),
				},
				expectResult:  "added a, b, c; deleted bar, baz, foo",
				expectChanged: true,
			},
			{
				name: "intersecting keys",
				keysOld: []resource.PropertyKey{
					resource.PropertyKey("both"),
					resource.PropertyKey("old"),
				},
				keysNew: []resource.PropertyKey{
					resource.PropertyKey("both"),
					resource.PropertyKey("new"),
				},
				expectResult:  "added new; deleted old",
				expectChanged: true,
			},
			{
				name: "same keys - same order",
				keysOld: []resource.PropertyKey{
					resource.PropertyKey("foo"),
					resource.PropertyKey("bar"),
					resource.PropertyKey("baz"),
				},
				keysNew: []resource.PropertyKey{
					resource.PropertyKey("foo"),
					resource.PropertyKey("bar"),
					resource.PropertyKey("baz"),
				},
			},
			{
				name: "same keys - different order",
				keysOld: []resource.PropertyKey{
					resource.PropertyKey("foo"),
					resource.PropertyKey("bar"),
					resource.PropertyKey("baz"),
				},
				keysNew: []resource.PropertyKey{
					resource.PropertyKey("baz"),
					resource.PropertyKey("bar"),
					resource.PropertyKey("foo"),
				},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{}
				res, changed := rp.diffPropertyKeys(tt.keysOld, tt.keysNew)
				assert.Equal(t, tt.expectChanged, changed)
				assert.Equal(t, tt.expectResult, res)
			})
		}
	})
	t.Run("diffAliases", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name          string
			keysOld       []resource.Alias
			keysNew       []resource.Alias
			expectResult  string
			expectChanged bool
		}{
			{
				name: "mutually exclusive keys",
				keysOld: []resource.Alias{
					{Name: "foo"},
					{Name: "bar"},
					{Name: "baz"},
				},
				keysNew: []resource.Alias{
					{Name: "a"},
					{Name: "b"},
					{Name: "c"},
				},
				expectResult: "added { a     false}, { b     false}, { c     false}; " +
					"deleted { bar     false}, { baz     false}, { foo     false}",
				expectChanged: true,
			},
			{
				name: "intersecting keys",
				keysOld: []resource.Alias{
					{Name: "both"},
					{Name: "old"},
				},
				keysNew: []resource.Alias{
					{Name: "both"},
					{Name: "new"},
				},
				expectResult:  "added { new     false}; deleted { old     false}",
				expectChanged: true,
			},
			{
				name: "same keys - same order",
				keysOld: []resource.Alias{
					{Name: "foo"},
					{Name: "bar"},
					{Name: "baz"},
				},
				keysNew: []resource.Alias{
					{Name: "foo"},
					{Name: "bar"},
					{Name: "baz"},
				},
			},
			{
				name: "same keys - different order",
				keysOld: []resource.Alias{
					{Name: "foo"},
					{Name: "bar"},
					{Name: "baz"},
				},
				keysNew: []resource.Alias{
					{Name: "baz"},
					{Name: "bar"},
					{Name: "foo"},
				},
			},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{}
				res, changed := rp.diffAliases(tt.keysOld, tt.keysNew)
				assert.Equal(t, tt.expectChanged, changed)
				assert.Equal(t, tt.expectResult, res)
			})
		}
	})
	t.Run("checkGoal", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			rp := &ResourcePlan{
				Goal: &GoalPlan{},
			}
			err := rp.checkGoal(
				resource.PropertyMap{},
				resource.PropertyMap{},
				&resource.Goal{})
			assert.NoError(t, err)
		})
		t.Run("violations", func(t *testing.T) {
			t.Run("custom mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Custom: true,
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Custom: false,
					})
				assert.ErrorContains(t, err, "resource kind changed (expected custom)")
			})
			t.Run("invalid provider reference", func(t *testing.T) {
				t.Parallel()
				t.Run("resource plan", func(t *testing.T) {
					t.Parallel()
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							Provider: "bad-provider",
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							Provider: "urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2::provider-foo",
						})
					assert.ErrorContains(t, err, "failed to parse provider reference")
				})
				t.Run("goal", func(t *testing.T) {
					t.Parallel()
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							Provider: "urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2::provider-bar",
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							Provider: "bad-provider",
						})
					assert.ErrorContains(t, err, "failed to parse provider reference")
				})
			})
			t.Run("provider", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Provider: "urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2::provider-bar",
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Provider: "urn:pulumi:dev::random::pulumi:providers:random::default_4_13_2::provider-foo",
					})
				assert.ErrorContains(t, err, "provider changed")
			})
			t.Run("parent mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Parent: "foo",
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Parent: "bar",
					})
				assert.ErrorContains(t, err, "parent changed")
			})
			t.Run("protect mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Protect: true,
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Protect: nil,
					})
				assert.ErrorContains(t, err, "protect changed")
			})
			t.Run("deleteBeforeReplace mismatch", func(t *testing.T) {
				t.Parallel()
				t.Run("both non-nil", func(t *testing.T) {
					t.Parallel()
					var planRef bool
					goalRef := true
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							DeleteBeforeReplace: &planRef,
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							DeleteBeforeReplace: &goalRef,
						})
					assert.ErrorContains(t, err, "deleteBeforeReplace changed")
				})
				t.Run("plan non-nil", func(t *testing.T) {
					t.Parallel()
					var planRef bool
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							DeleteBeforeReplace: &planRef,
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{})
					assert.ErrorContains(t, err, "deleteBeforeReplace changed (expected false)")
				})
				t.Run("goal non-nil", func(t *testing.T) {
					t.Parallel()
					var goalRef bool
					rp := &ResourcePlan{
						Goal: &GoalPlan{},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							DeleteBeforeReplace: &goalRef,
						})
					assert.ErrorContains(t, err, "deleteBeforeReplace changed (expected no value)")
				})
			})
			t.Run("id mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						ID: "foo",
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						ID: "bar",
					})
				assert.ErrorContains(t, err, "importID changed")
			})
			t.Run("customTimeouts mismatch", func(t *testing.T) {
				t.Run("create", func(t *testing.T) {
					t.Parallel()
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							CustomTimeouts: resource.CustomTimeouts{
								Create: 10,
							},
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							CustomTimeouts: resource.CustomTimeouts{
								Create: 5,
							},
						})
					assert.ErrorContains(t, err, "create timeout changed")
				})
				t.Run("update", func(t *testing.T) {
					t.Parallel()
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							CustomTimeouts: resource.CustomTimeouts{
								Update: 10,
							},
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							CustomTimeouts: resource.CustomTimeouts{
								Update: 5,
							},
						})
					assert.ErrorContains(t, err, "update timeout changed")
				})
				t.Run("delete", func(t *testing.T) {
					t.Parallel()
					rp := &ResourcePlan{
						Goal: &GoalPlan{
							CustomTimeouts: resource.CustomTimeouts{
								Delete: 10,
							},
						},
					}
					err := rp.checkGoal(
						resource.PropertyMap{},
						resource.PropertyMap{},
						&resource.Goal{
							CustomTimeouts: resource.CustomTimeouts{
								Delete: 5,
							},
						})
					assert.ErrorContains(t, err, "delete timeout changed")
				})
			})
			t.Run("ignoreChanges mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						IgnoreChanges: []string{
							"foo",
						},
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						IgnoreChanges: []string{
							"bar",
						},
					})
				assert.ErrorContains(t, err, "ignoreChanges changed")
			})
			t.Run("additionalSecretOutputs mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						AdditionalSecretOutputs: []resource.PropertyKey{
							"foo",
						},
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						AdditionalSecretOutputs: []resource.PropertyKey{
							"bar",
						},
					})
				assert.ErrorContains(t, err, "additionalSecretOutputs changed")
			})
			t.Run("dependencies mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Dependencies: []resource.URN{
							"foo",
						},
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Dependencies: []resource.URN{
							"bar",
						},
					})
				assert.ErrorContains(t, err, "dependencies changed")
			})
			t.Run("aliases mismatch", func(t *testing.T) {
				t.Parallel()
				rp := &ResourcePlan{
					Goal: &GoalPlan{
						Aliases: []resource.Alias{
							{Name: "foo"},
						},
					},
				}
				err := rp.checkGoal(
					resource.PropertyMap{},
					resource.PropertyMap{},
					&resource.Goal{
						Aliases: []resource.Alias{
							{Name: "bar"},
						},
					})
				assert.ErrorContains(t, err, "aliases changed")
			})
		})
	})
}

func TestCheckDiff(t *testing.T) {
	t.Parallel()
	t.Run("planDiff.Deletes", func(t *testing.T) {
		t.Parallel()
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			err := checkDiff(
				resource.PropertyMap{},
				resource.PropertyMap{},
				PlanDiff{
					Deletes: []resource.PropertyKey{
						"should-be-here",
					},
				})
			assert.NoError(t, err)
		})
		t.Run("diff violation", func(t *testing.T) {
			t.Parallel()
			t.Run("add", func(t *testing.T) {
				t.Parallel()
				err := checkDiff(
					resource.PropertyMap{},
					resource.PropertyMap{
						resource.PropertyKey("should-delete"): resource.NewStringProperty("test"),
					},
					PlanDiff{
						Deletes: []resource.PropertyKey{
							"should-delete",
						},
					})
				assert.Error(t, err)
			})
			t.Run("update", func(t *testing.T) {
				t.Parallel()
				err := checkDiff(
					resource.PropertyMap{
						resource.PropertyKey("should-delete"): resource.NewStringProperty("test"),
					},
					resource.PropertyMap{
						resource.PropertyKey("should-delete"): resource.NewStringProperty("new-test"),
					},
					PlanDiff{
						Deletes: []resource.PropertyKey{
							"should-delete",
						},
					})
				assert.Error(t, err)
			})
			t.Run("same", func(t *testing.T) {
				t.Parallel()
				t.Skip("difficulty covering this")
			})
		})
	})
	t.Run("planDiff.Adds", func(t *testing.T) {
		t.Parallel()
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			err := checkDiff(
				resource.PropertyMap{},
				resource.PropertyMap{
					resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
				},
				PlanDiff{
					Adds: resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
				})
			assert.NoError(t, err)
		})
		t.Run("diff violation", func(t *testing.T) {
			t.Parallel()
			t.Skip("TODO")
		})
	})
	t.Run("planDiff.Updates", func(t *testing.T) {
		t.Parallel()
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			err := checkDiff(
				resource.PropertyMap{
					resource.PropertyKey("should-update"): resource.NewStringProperty("old-test"),
				},
				resource.PropertyMap{
					resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
				},
				PlanDiff{
					Updates: resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
				})
			assert.NoError(t, err)
		})
		t.Run("ok same", func(t *testing.T) {
			t.Parallel()
			err := checkDiff(
				resource.PropertyMap{
					resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
				},
				resource.PropertyMap{
					resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
				},
				PlanDiff{
					Updates: resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
				})
			assert.NoError(t, err)
		})
		t.Run("diff violation", func(t *testing.T) {
			t.Parallel()
			t.Run("add", func(t *testing.T) {
				t.Parallel()
				err := checkDiff(
					resource.PropertyMap{},
					resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
					PlanDiff{
						Updates: resource.PropertyMap{
							resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
						},
					})
				assert.ErrorContains(t, err,
					"properties changed: ~+should-update[{new-test}!={new-test}], ~+should-update[{new-test}!={new-test}]")
			})
			t.Run("same", func(t *testing.T) {
				t.Parallel()
				err := checkDiff(
					resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
					resource.PropertyMap{
						resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
					},
					PlanDiff{
						Updates: resource.PropertyMap{
							resource.PropertyKey("should-update"): resource.NewStringProperty("new-new-test"),
						},
					})
				assert.ErrorContains(t, err, "properties changed: ~=should-update[{new-new-test}!={new-test}]")
			})
			t.Run("missing", func(t *testing.T) {
				t.Parallel()
				t.Run("deleted, instead of updated", func(t *testing.T) {
					t.Parallel()
					err := checkDiff(
						resource.PropertyMap{
							resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
						},
						resource.PropertyMap{},
						PlanDiff{
							Updates: resource.PropertyMap{
								resource.PropertyKey("should-update"): resource.NewStringProperty("new-new-test"),
							},
						})
					assert.ErrorContains(t, err,
						"properties changed: ~-should-update[{new-new-test}], ~-should-update[{new-new-test}]")
				})
				t.Run("computed", func(t *testing.T) {
					t.Parallel()
					err := checkDiff(
						resource.PropertyMap{
							resource.PropertyKey("should-update"): resource.NewStringProperty("new-test"),
						},
						resource.PropertyMap{},
						PlanDiff{
							Updates: resource.PropertyMap{
								resource.PropertyKey("should-update"): resource.MakeComputed(resource.NewStringProperty("new-new-test")),
							},
						})
					assert.NoError(t, err)
				})
			})
		})
	})
}
