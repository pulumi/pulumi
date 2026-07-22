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

// GCP's role names are their own identifiers, so its presets map to themselves and a custom
// predefined role needs no translation.
func TestResolvePolicy_GCPPresetsMapToThemselves(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}

	got, err := s.resolvePolicy("admin", gcpPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "roles/editor", got)

	got, err = s.resolvePolicy("roles/viewer", gcpPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "roles/viewer", got)

	got, err = s.resolvePolicy("roles/storage.admin", gcpPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "roles/storage.admin", got)
}
