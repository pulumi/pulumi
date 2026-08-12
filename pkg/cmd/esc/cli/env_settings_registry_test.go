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

package cli

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvSettingsRegistry(t *testing.T) {
	t.Parallel()
	t.Run("GetSetting", func(t *testing.T) {
		t.Parallel()
		registry := NewEnvSettingsRegistry()

		t.Run("valid setting", func(t *testing.T) {
			t.Parallel()
			setting, ok := registry.GetSetting("deletion-protected")
			assert.True(t, ok)
			require.NotNil(t, setting)
		})

		t.Run("unknown setting", func(t *testing.T) {
			t.Parallel()
			_, ok := registry.GetSetting("unknown-setting")
			assert.False(t, ok)
		})
	})

	t.Run("EndToEnd", func(t *testing.T) {
		t.Parallel()
		registry := NewEnvSettingsRegistry()

		setting, ok := registry.GetSetting("deletion-protected")
		assert.True(t, ok)

		value, err := setting.ValidateValue("true")
		require.NoError(t, err)
		assert.Equal(t, true, value)

		req := client.PatchEnvironmentSettingsRequest{}
		setting.SetValue(&req, value)
		require.NotNil(t, req.DeletionProtected)
		assert.True(t, *req.DeletionProtected)

		settings := &client.EnvironmentSettings{DeletionProtected: true}
		assert.Equal(t, true, setting.GetValue(settings))
	})
}
