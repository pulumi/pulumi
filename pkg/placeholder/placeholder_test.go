package placeholder

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestGetDefaultOrg(t *testing.T) {
	ctx := context.Background()
	userConfiguredOrg := "user-configured-default-org"
	backendConfiguredOrg := "backend-configured-org"
	t.Run("prefers user-configured default org", func(t *testing.T) {
		// GIVEN
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return userConfiguredOrg, nil
		}

		testBackend := &backend.MockBackend{
			GetDefaultOrgF: func(ctx context.Context) (string, error) {
				assert.Fail(t, "should not make api call for get default org")
				return "", nil
			},
		}

		// WHEN
		org, err := getDefaultOrg(ctx, testBackend, nil, defaultOrgConfigLookupFunc)

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, userConfiguredOrg, org)
	})

	t.Run("falls back to making a call for user org", func(t *testing.T) {
		// GIVEN
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return "", nil
		}

		testBackend := &backend.MockBackend{
			GetDefaultOrgF: func(ctx context.Context) (string, error) {
				return backendConfiguredOrg, nil
			},
		}

		// WHEN
		org, err := getDefaultOrg(ctx, testBackend, nil, defaultOrgConfigLookupFunc)

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, backendConfiguredOrg, org)
	})

	// This maintains existing behavior with `GetBackendConfigDefaultOrg`.
	t.Run("returns empty string if nothing is configured", func(t *testing.T) {
		// GIVEN
		defaultOrgConfigLookupFunc := func(*workspace.Project) (string, error) {
			return "", nil
		}

		testBackend := &backend.MockBackend{}

		// WHEN
		org, err := getDefaultOrg(ctx, testBackend, nil, defaultOrgConfigLookupFunc)

		// THEN
		assert.NoError(t, err)
		assert.Equal(t, "", org)
	})
}
