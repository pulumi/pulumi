// Copyright 2025, Pulumi Corporation.

package cli

import (
	"testing"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/stretchr/testify/assert"
)

func TestEnvSettingsRegistry(t *testing.T) {
	t.Run("GetSetting", func(t *testing.T) {
		registry := NewEnvSettingsRegistry()

		t.Run("valid setting", func(t *testing.T) {
			setting, ok := registry.GetSetting("deletion-protected")
			assert.True(t, ok)
			assert.NotNil(t, setting)
		})

		t.Run("unknown setting", func(t *testing.T) {
			_, ok := registry.GetSetting("unknown-setting")
			assert.False(t, ok)
		})
	})

	t.Run("EndToEnd", func(t *testing.T) {
		registry := NewEnvSettingsRegistry()

		setting, ok := registry.GetSetting("deletion-protected")
		assert.True(t, ok)

		value, err := setting.ValidateValue("true")
		assert.NoError(t, err)
		assert.Equal(t, true, value)

		req := client.PatchEnvironmentSettingsRequest{}
		setting.SetValue(&req, value)
		assert.NotNil(t, req.DeletionProtected)
		assert.True(t, *req.DeletionProtected)

		settings := &client.EnvironmentSettings{DeletionProtected: true}
		assert.Equal(t, true, setting.GetValue(settings))
	})
}
