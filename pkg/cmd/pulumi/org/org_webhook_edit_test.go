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

func TestRemoveFromSlice(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]string{"b"},
		removeFromSlice([]string{"a", "b", "c"}, []string{"a", "c"}))

	// Removing nonexistent is a no-op.
	assert.Equal(t,
		[]string{"a"},
		removeFromSlice([]string{"a"}, []string{"z"}))

	// Empty remove.
	assert.Equal(t,
		[]string{"a"},
		removeFromSlice([]string{"a"}, nil))
}

func TestAddToSlice(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]string{"a", "b"},
		addToSlice([]string{"a"}, []string{"b"}))

	// Duplicate is idempotent.
	assert.Equal(t,
		[]string{"a"},
		addToSlice([]string{"a"}, []string{"a"}))

	// Add to empty.
	assert.Equal(t,
		[]string{"x"},
		addToSlice(nil, []string{"x"}))
}

func TestOrgWebhookEditCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newOrgWebhookEditCmd()
	assert.Contains(t, cmd.Use, "edit")
	require.NotNil(t, cmd.RunE)

	for _, name := range []string{
		"org", "url", "hook-format",
		"add-event", "remove-event",
		"add-group", "remove-group",
		"active", "secret", "display-name", "output",
	} {
		f := cmd.Flags().Lookup(name)
		require.NotNil(t, f, "expected flag %q", name)
	}
}
