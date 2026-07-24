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

// Azure assigns roles by definition ID, so the presets resolve to GUIDs and a custom role is
// passed through as a GUID rather than a name.
func TestResolvePolicy_AzureResolvesToRoleDefinitionIDs(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}

	got, err := s.resolvePolicy("Contributor", azurePolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "b24988ac-6180-42a0-ab88-20f7382dd24c", got)

	got, err = s.resolvePolicy("readonly", azurePolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "acdd72a7-3385-48ef-bd42-f606fba81ae7", got)

	custom := "00000000-0000-0000-0000-000000000001"
	got, err = s.resolvePolicy(custom, azurePolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, custom, got)
}
