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

package org

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOrgGroupsAndEvents(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		err := validateOrgGroupsAndEvents(
			[]string{"stacks", "environments"},
			[]string{"deployment_queued"},
		)
		require.NoError(t, err)
	})

	t.Run("invalid group", func(t *testing.T) {
		t.Parallel()
		err := validateOrgGroupsAndEvents([]string{"bogus"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid group "bogus"`)
	})

	t.Run("invalid event", func(t *testing.T) {
		t.Parallel()
		err := validateOrgGroupsAndEvents(nil, []string{"not_real"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid event "not_real"`)
	})

	t.Run("event covered by group", func(t *testing.T) {
		t.Parallel()
		err := validateOrgGroupsAndEvents(
			[]string{"stacks"}, []string{"update_succeeded"},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `already included by group "stacks"`)
	})

	t.Run("org-specific groups valid", func(t *testing.T) {
		t.Parallel()
		err := validateOrgGroupsAndEvents(
			[]string{"environments", "change_requests"}, nil,
		)
		require.NoError(t, err)
	})
}

func TestOrgFiltersNotCoveredByGroups(t *testing.T) {
	t.Parallel()

	// All groups → no remaining.
	assert.Empty(t, orgFiltersNotCoveredByGroups(orgWebhookGroups))

	// Only stacks → other groups' filters remain.
	remaining := orgFiltersNotCoveredByGroups([]string{"stacks"})
	assert.Contains(t, remaining, "deployment_queued")
	assert.Contains(t, remaining, "environment_created")
	assert.NotContains(t, remaining, "update_succeeded")
}
