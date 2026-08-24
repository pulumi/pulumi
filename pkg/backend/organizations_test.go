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

package backend

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBackendConfigDefaultOrg(t *testing.T) {
	t.Run("prefers PULUMI_DEFAULT_ORGANIZATION over configured default org", func(t *testing.T) {
		t.Setenv("PULUMI_HOME", t.TempDir())
		t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "env-org")
		require.NoError(t, workspace.SetBackendConfigDefaultOrg("https://api.example.com", "configured-org"))

		org, err := getBackendConfigDefaultOrg(nil)
		require.NoError(t, err)
		assert.Equal(t, "env-org", org)
	})

	t.Run("falls back to configured default org", func(t *testing.T) {
		t.Setenv("PULUMI_HOME", t.TempDir())
		t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "")
		require.NoError(t, workspace.SetBackendConfigDefaultOrg("https://api.example.com", "configured-org"))

		org, err := getBackendConfigDefaultOrg(nil)
		require.NoError(t, err)
		assert.Equal(t, "configured-org", org)
	})
}

func TestGetLegacyDefaultOrgFallback(t *testing.T) {
	t.Parallel()

	t.Run("returns empty string for backends that do not support orgs", func(t *testing.T) {
		t.Parallel()

		testBackend := &MockBackend{
			SupportsOrganizationsF: func() bool {
				return false
			},
		}
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return "", nil
		}

		org, err := getLegacyDefaultOrgFallback(testBackend, nil, defaultOrgConfigLookupFunc)
		require.NoError(t, err)
		assert.Empty(t, org)
	})

	t.Run("returns empty string if user has default org configured", func(t *testing.T) {
		t.Parallel()

		testBackend := &MockBackend{
			SupportsOrganizationsF: func() bool {
				return true
			},
		}
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return "some-configured-org", nil
		}

		org, err := getLegacyDefaultOrgFallback(testBackend, nil, defaultOrgConfigLookupFunc)
		require.NoError(t, err)
		assert.Empty(t, org)
	})

	t.Run("returns user org as legacy fallback behavior", func(t *testing.T) {
		t.Parallel()
		user := "test-user"
		testBackend := &MockBackend{
			SupportsOrganizationsF: func() bool {
				return true
			},
			CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
				return user, nil, nil, nil
			},
		}
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return "", nil
		}

		org, err := getLegacyDefaultOrgFallback(testBackend, nil, defaultOrgConfigLookupFunc)
		require.NoError(t, err)
		assert.Equal(t, user, org)
	})
}
