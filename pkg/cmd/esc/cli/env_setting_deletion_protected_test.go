// Copyright 2025, Pulumi Corporation.

package cli

import (
	"testing"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/stretchr/testify/assert"
)

func TestDeletionProtectedSetting(t *testing.T) {
	s := &DeletionProtectedSetting{}

	t.Run("ValidateValue", func(t *testing.T) {
		t.Run("valid true", func(t *testing.T) {
			value, err := s.ValidateValue("true")
			assert.NoError(t, err)
			assert.Equal(t, true, value)
		})

		t.Run("valid false", func(t *testing.T) {
			value, err := s.ValidateValue("false")
			assert.NoError(t, err)
			assert.Equal(t, false, value)
		})

		t.Run("invalid value", func(t *testing.T) {
			_, err := s.ValidateValue("yes")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid value for deletion-protected")
		})

		t.Run("case sensitive", func(t *testing.T) {
			_, err := s.ValidateValue("True")
			assert.Error(t, err)
		})
	})

	t.Run("GetSetValue", func(t *testing.T) {
		t.Run("true", func(t *testing.T) {
			req := &client.PatchEnvironmentSettingsRequest{}
			s.SetValue(req, true)
			assert.NotNil(t, req.DeletionProtected)
			assert.True(t, *req.DeletionProtected)

			settings := &client.EnvironmentSettings{DeletionProtected: true}
			assert.Equal(t, true, s.GetValue(settings))
		})

		t.Run("false", func(t *testing.T) {
			req := &client.PatchEnvironmentSettingsRequest{}
			s.SetValue(req, false)
			assert.NotNil(t, req.DeletionProtected)
			assert.False(t, *req.DeletionProtected)

			settings := &client.EnvironmentSettings{DeletionProtected: false}
			assert.Equal(t, false, s.GetValue(settings))
		})
	})
}
