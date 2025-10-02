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

package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaintedResourceIsTargetedForReplace verifies that tainted resources are correctly identified for replacement
func TestTaintedResourceIsTargetedForReplace(t *testing.T) {
	t.Parallel()

	// Create a deployment with no explicit replace targets
	deployment := &Deployment{
		opts: &Options{},
	}

	sg := &stepGenerator{
		deployment: deployment,
	}

	urn := resource.NewURN("test", "test", "", "test:test:Test", "myresource")

	// Test 1: Non-tainted resource should not be targeted for replace
	oldState := &resource.State{
		URN:   urn,
		Taint: false,
	}
	assert.False(t, sg.isTargetedReplace(urn, oldState), "Non-tainted resource should not be targeted for replace")

	// Test 2: Tainted resource should be targeted for replace
	oldState.Taint = true
	assert.True(t, sg.isTargetedReplace(urn, oldState), "Tainted resource should be targeted for replace")

	// Test 3: Nil old state should not be targeted for replace
	assert.False(t, sg.isTargetedReplace(urn, nil), "Nil old state should not be targeted for replace")
}

// TestTaintedResourceWithExplicitReplaceTarget verifies that both taint and explicit replace targets work
func TestTaintedResourceWithExplicitReplaceTarget(t *testing.T) {
	t.Parallel()

	urn1 := resource.NewURN("test", "test", "", "test:test:Test", "resource1")
	urn2 := resource.NewURN("test", "test", "", "test:test:Test", "resource2")

	// Create a deployment with explicit replace target for resource1
	deployment := &Deployment{
		opts: &Options{
			ReplaceTargets: NewUrnTargets([]string{string(urn1)}),
		},
	}

	sg := &stepGenerator{
		deployment: deployment,
	}

	// Test 1: Explicitly targeted resource should be replaced even if not tainted
	oldState1 := &resource.State{
		URN:   urn1,
		Taint: false,
	}
	assert.True(t, sg.isTargetedReplace(urn1, oldState1), "Explicitly targeted resource should be replaced")

	// Test 2: Tainted resource should be replaced even if not explicitly targeted
	oldState2 := &resource.State{
		URN:   urn2,
		Taint: true,
	}
	assert.True(t, sg.isTargetedReplace(urn2, oldState2), "Tainted resource should be replaced")

	// Test 3: Resource that is both tainted and explicitly targeted should be replaced
	oldState1.Taint = true
	assert.True(t, sg.isTargetedReplace(urn1, oldState1), "Resource both tainted and targeted should be replaced")

	// Test 4: Resource that is neither tainted nor targeted should not be replaced
	oldState2.Taint = false
	assert.False(t, sg.isTargetedReplace(urn2, oldState2), "Resource neither tainted nor targeted should not be replaced")
}

// TestTaintedResourceDiff verifies that tainted resources trigger a diff with replacement
func TestTaintedResourceDiff(t *testing.T) {
	t.Parallel()

	deployment := &Deployment{
		opts: &Options{},
	}

	sg := &stepGenerator{
		deployment: deployment,
	}

	urn := resource.NewURN("test", "test", "", "test:test:Test", "myresource")
	inputs := resource.PropertyMap{
		"prop1": resource.NewProperty("value1"),
	}
	outputs := resource.PropertyMap{
		"prop1": resource.NewProperty("value1"),
	}

	// Create a tainted resource
	oldState := resource.NewState{
		Type:                    urn.Type(),
		URN:                     urn,
		Custom:                  false,
		Delete:                  false,
		ID:                      "",
		Inputs:                  inputs,
		Outputs:                 outputs,
		Parent:                  "",
		Protect:                 false,
		Taint:                   true,
		External:                false,
		Dependencies:            nil,
		InitErrors:              nil,
		Provider:                "",
		PropertyDependencies:    nil,
		PendingReplacement:      false,
		AdditionalSecretOutputs: nil,
		Aliases:                 nil,
		CustomTimeouts:          nil,
		ImportID:                "",
		RetainOnDelete:          false,
		DeletedWith:             "",
		Created:                 nil,
		Modified:                nil,
		SourcePosition:          "",
		StackTrace:              nil,
		IgnoreChanges:           nil,
		ReplaceOnChanges:        nil,
		RefreshBeforeUpdate:     false,
		ViewOf:                  "",
		ResourceHooks:           nil,
	}.Make()
	done := make(chan *RegisterResult)
	event := &registerResourceEvent{
		goal: resource.NewGoal{
			Type:                    urn.Type(),
			Name:                    urn.Name(),
			Custom:                  true,
			Properties:              inputs,
			Parent:                  "",
			Protect:                 nil,
			Dependencies:            nil,
			Provider:                "",
			InitErrors:              []string{},
			PropertyDependencies:    nil,
			DeleteBeforeReplace:     nil,
			IgnoreChanges:           nil,
			AdditionalSecretOutputs: nil,
			Aliases:                 nil,
			ID:                      "",
			CustomTimeouts:          nil,
			ReplaceOnChanges:        nil,
			RetainOnDelete:          nil,
			DeletedWith:             "",
			SourcePosition:          "",
			StackTrace:              nil,
			ResourceHooks:           nil,
		}.Make(),
		done: done,
	}

	// Call diff - it should return a replace diff due to taint
	result, _, err := sg.diff(event, event.Goal(), nil, []byte{}, urn, oldState, oldState, inputs, inputs, nil)
	require.NoError(t, err)

	// Should indicate changes and have replacement keys
	assert.Equal(t, plugin.DiffSome, result.Changes, "Tainted resource should show changes")
	assert.Contains(t, result.ReplaceKeys, resource.PropertyKey("id"),
		"Tainted resource should have 'id' in replace keys")
}

// TestTaintedProviderResource verifies that provider resources can be tainted
func TestTaintedProviderResource(t *testing.T) {
	t.Parallel()

	deployment := &Deployment{
		opts: &Options{},
	}

	sg := &stepGenerator{
		deployment: deployment,
	}

	// Create a tainted provider resource
	providerURN := resource.NewURN("test", "test", "", "pulumi:providers:aws", "default")
	providerState := &resource.State{
		URN:    providerURN,
		Type:   providers.MakeProviderType("aws"),
		Custom: true,
		Taint:  true,
	}

	// Provider resources can be tainted and should be targeted for replacement
	assert.True(t, sg.isTargetedReplace(providerURN, providerState), "Tainted provider should be targeted for replace")
}

// TestTaintInteractionWithReplaceTargets verifies taint and replace targets work together correctly
func TestTaintInteractionWithReplaceTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tainted        bool
		explicitTarget bool
		expectReplace  bool
	}{
		{
			name:           "neither tainted nor targeted",
			tainted:        false,
			explicitTarget: false,
			expectReplace:  false,
		},
		{
			name:           "only tainted",
			tainted:        true,
			explicitTarget: false,
			expectReplace:  true,
		},
		{
			name:           "only targeted",
			tainted:        false,
			explicitTarget: true,
			expectReplace:  true,
		},
		{
			name:           "both tainted and targeted",
			tainted:        true,
			explicitTarget: true,
			expectReplace:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			urn := resource.NewURN("test", "test", "", "test:test:Test", "myresource")

			var replaceTargets UrnTargets
			if tt.explicitTarget {
				replaceTargets = NewUrnTargets([]string{string(urn)})
			}

			deployment := &Deployment{
				opts: &Options{
					ReplaceTargets: replaceTargets,
				},
			}

			sg := &stepGenerator{
				deployment: deployment,
			}

			oldState := &resource.State{
				URN:   urn,
				Taint: tt.tainted,
			}

			result := sg.isTargetedReplace(urn, oldState)
			assert.Equal(t, tt.expectReplace, result,
				"Expected replace=%v for tainted=%v, explicitTarget=%v",
				tt.expectReplace, tt.tainted, tt.explicitTarget)
		})
	}
}

// TestTaintWithNilOldState verifies that nil old state is handled correctly
func TestTaintWithNilOldState(t *testing.T) {
	t.Parallel()

	deployment := &Deployment{
		opts: &Options{},
	}

	sg := &stepGenerator{
		deployment: deployment,
	}

	urn := resource.NewURN("test", "test", "", "test:test:Test", "myresource")

	// Nil old state should not cause panic and should return false
	assert.False(t, sg.isTargetedReplace(urn, nil), "Nil old state should not be targeted for replace")
}

// TestDiffWithTaintedResource verifies the diff function handles tainted resources correctly
func TestDiffWithTaintedResource(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("test", "test", "", "test:test:Test", "myresource")

	tests := []struct {
		name          string
		tainted       bool
		expectReplace bool
	}{
		{
			name:          "non-tainted resource",
			tainted:       false,
			expectReplace: false,
		},
		{
			name:          "tainted resource",
			tainted:       true,
			expectReplace: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deployment := &Deployment{
				opts: &Options{},
			}

			sg := &stepGenerator{
				deployment: deployment,
			}

			inputs := resource.PropertyMap{
				"prop1": resource.NewProperty("value1"),
			}
			outputs := resource.PropertyMap{
				"prop1": resource.NewProperty("value1"),
			}

			// Create a tainted resource
			oldState := resource.NewState{
				Type:                    urn.Type(),
				URN:                     urn,
				Custom:                  false,
				Delete:                  false,
				ID:                      "",
				Inputs:                  inputs,
				Outputs:                 outputs,
				Parent:                  "",
				Protect:                 false,
				Taint:                   tt.tainted,
				External:                false,
				Dependencies:            nil,
				InitErrors:              nil,
				Provider:                "",
				PropertyDependencies:    nil,
				PendingReplacement:      false,
				AdditionalSecretOutputs: nil,
				Aliases:                 nil,
				CustomTimeouts:          nil,
				ImportID:                "",
				RetainOnDelete:          false,
				DeletedWith:             "",
				Created:                 nil,
				Modified:                nil,
				SourcePosition:          "",
				StackTrace:              nil,
				IgnoreChanges:           nil,
				ReplaceOnChanges:        nil,
				RefreshBeforeUpdate:     false,
				ViewOf:                  "",
				ResourceHooks:           nil,
			}.Make()
			done := make(chan *RegisterResult)
			event := &registerResourceEvent{
				goal: resource.NewGoal{
					Type:                    urn.Type(),
					Name:                    urn.Name(),
					Custom:                  true,
					Properties:              inputs,
					Parent:                  "",
					Protect:                 nil,
					Dependencies:            nil,
					Provider:                "",
					InitErrors:              []string{},
					PropertyDependencies:    nil,
					DeleteBeforeReplace:     nil,
					IgnoreChanges:           nil,
					AdditionalSecretOutputs: nil,
					Aliases:                 nil,
					ID:                      "",
					CustomTimeouts:          nil,
					ReplaceOnChanges:        nil,
					RetainOnDelete:          nil,
					DeletedWith:             "",
					SourcePosition:          "",
					StackTrace:              nil,
					ResourceHooks:           nil,
				}.Make(),
				done: done,
			}

			// When no provider is specified, diff should still handle tainted resources
			result, _, err := sg.diff(event, event.Goal(), nil, []byte{}, urn, oldState, oldState, inputs, inputs, nil)
			require.NoError(t, err)

			if tt.expectReplace {
				assert.Equal(t, plugin.DiffSome, result.Changes, "Tainted resource should show changes")
				assert.Contains(t, result.ReplaceKeys, resource.PropertyKey("id"),
					"Tainted resource should have 'id' in replace keys")
			} else {
				// When not tainted and no provider, diff returns no changes
				assert.Equal(t, plugin.DiffNone, result.Changes,
					"Non-tainted resource with no changes should show no diff")
			}
		})
	}
}
