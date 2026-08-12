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

func TestDeletionProtectedSetting(t *testing.T) {
	t.Parallel()
	s := &DeletionProtectedSetting{}

	t.Run("ValidateValue", func(t *testing.T) {
		t.Parallel()
		t.Run("valid true", func(t *testing.T) {
			t.Parallel()
			value, err := s.ValidateValue("true")
			require.NoError(t, err)
			assert.Equal(t, true, value)
		})

		t.Run("valid false", func(t *testing.T) {
			t.Parallel()
			value, err := s.ValidateValue("false")
			require.NoError(t, err)
			assert.Equal(t, false, value)
		})

		t.Run("invalid value", func(t *testing.T) {
			t.Parallel()
			_, err := s.ValidateValue("yes")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid value for deletion-protected")
		})

		t.Run("case sensitive", func(t *testing.T) {
			t.Parallel()
			_, err := s.ValidateValue("True")
			assert.Error(t, err)
		})
	})

	t.Run("GetSetValue", func(t *testing.T) {
		t.Parallel()
		t.Run("true", func(t *testing.T) {
			t.Parallel()
			req := &client.PatchEnvironmentSettingsRequest{}
			s.SetValue(req, true)
			require.NotNil(t, req.DeletionProtected)
			assert.True(t, *req.DeletionProtected)

			settings := &client.EnvironmentSettings{DeletionProtected: true}
			assert.Equal(t, true, s.GetValue(settings))
		})

		t.Run("false", func(t *testing.T) {
			t.Parallel()
			req := &client.PatchEnvironmentSettingsRequest{}
			s.SetValue(req, false)
			require.NotNil(t, req.DeletionProtected)
			assert.False(t, *req.DeletionProtected)

			settings := &client.EnvironmentSettings{DeletionProtected: false}
			assert.Equal(t, false, s.GetValue(settings))
		})
	})
}
