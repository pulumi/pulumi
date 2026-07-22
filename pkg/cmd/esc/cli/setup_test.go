// Copyright 2026, Pulumi Corporation.
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

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The presets are synthetic so that these cases test the resolver itself rather than any one
// provider's names; each provider's own test covers what its presets resolve to.
var testPolicyChoices = []policyChoice{
	{name: "AdminPolicy", id: "admin-id", alias: policyAliasAdmin, desc: policyAdminAccess},
	{name: "ReadonlyPolicy", id: "readonly-id", alias: policyAliasReadonly, desc: policyReadonlyAccess},
}

// resolvePolicy only reaches for the command's colors when it prompts, so a zero-value
// setupCommand is enough to cover every case that resolves without prompting.
func TestResolvePolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		policy string
		want   string
	}{
		{"generic admin alias", "admin", "admin-id"},
		{"generic readonly alias", "readonly", "readonly-id"},
		{"official admin name", "AdminPolicy", "admin-id"},
		{"official readonly name", "ReadonlyPolicy", "readonly-id"},
		{"aliases are case-insensitive", "AdMiN", "admin-id"},
		{"official names are case-insensitive", "adminpolicy", "admin-id"},
		// The point of accepting custom policies: anything that is not a preset is a native
		// identifier and is handed to the provider untouched, casing included.
		{"custom value", "SomeCustomPolicy", "SomeCustomPolicy"},
		{
			"custom value that looks like a preset",
			"AdminPolicyPlus",
			"AdminPolicyPlus",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &setupCommand{}
			// --yes must not change how an explicit policy resolves; it only removes the prompt.
			for _, yes := range []bool{false, true} {
				got, err := s.resolvePolicy(c.policy, testPolicyChoices, yes)
				require.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}

// Each preset carries its own description, so the prompt cannot mislabel a choice by listing the
// presets in an unexpected order.
func TestPolicyChoiceLabel(t *testing.T) {
	t.Parallel()

	for _, c := range testPolicyChoices {
		assert.Equal(t, c.name+" - "+c.desc, c.label())
	}
}

// An unknown value is a custom policy even when no presets are offered at all.
func TestResolvePolicy_NoChoices(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}
	got, err := s.resolvePolicy("roles/custom", nil, false)
	require.NoError(t, err)
	assert.Equal(t, "roles/custom", got)

	// Omitted with nothing to prompt with is a programming error, not a panic.
	_, err = s.resolvePolicy("", nil, false)
	assert.Error(t, err)
}

// With --yes there is nobody to prompt, so an omitted policy is an error rather than a
// silent default: admin is needed for Deployments and readonly for Insights.
func TestResolvePolicy_OmittedWithYes(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}
	_, err := s.resolvePolicy("", testPolicyChoices, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--policy must be set when using --yes")
	// The message names the presets so the user has something to copy.
	assert.Contains(t, err.Error(), "AdminPolicy")
	assert.Contains(t, err.Error(), "ReadonlyPolicy")
}
