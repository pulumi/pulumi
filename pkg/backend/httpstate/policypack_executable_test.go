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

package httpstate

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackLocationPlatformSelection(t *testing.T) {
	t.Parallel()

	t.Run("legacy single location", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack", PackLocation: "https://legacy",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://legacy", loc)
	})

	t.Run("platform map picks host platform", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack",
			PackLocations: map[string]string{
				workspace.CurrentPlatform(): "https://mine",
				"made-up-platform":          "https://other",
			},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://mine", loc)
	})

	t.Run("host platform missing is a loud error", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "pack",
			PackLocations: map[string]string{"made-up-platform": "https://other"},
		}}
		_, err := rp.packLocation()
		require.Error(t, err)
		assert.ErrorContains(t, err, "pack")
		assert.ErrorContains(t, err, workspace.CurrentPlatform())
		assert.ErrorContains(t, err, "made-up-platform")
	})
}
