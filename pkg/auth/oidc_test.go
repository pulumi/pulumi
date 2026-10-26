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

package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthContextForTokenExchange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		organization string
		team         string
		user         string
		token        string
		expiration   string
		accessToken  string
		wantScope    string
		wantDuration time.Duration
		wantError    string
	}{
		{
			name: "organization", organization: "org", token: "oidc-token",
			wantDuration: 2 * time.Hour,
		},
		{
			name: "team", organization: "org", team: "team", token: "oidc-token",
			wantScope: "team:team", wantDuration: 2 * time.Hour,
		},
		{
			name: "user with custom expiration", organization: "org", user: "user", token: "oidc-token",
			expiration: "45m", wantScope: "user:user", wantDuration: 45 * time.Minute,
		},
		{
			name: "zero expiration", organization: "org", token: "oidc-token", expiration: "0s",
		},
		{
			name: "negative expiration", organization: "org", token: "oidc-token", expiration: "-1h",
			wantDuration: -time.Hour,
		},
		{
			name: "missing token", organization: "org",
			wantError: "oidc token must be specified for token exchange",
		},
		{
			name: "environment access token", organization: "org", token: "oidc-token", accessToken: "access-token",
			wantError: "cannot perform token exchange when an access token is set as environment variable",
		},
		{
			name: "missing organization", token: "oidc-token",
			wantError: "organization must be specified for token exchange",
		},
		{
			name: "team and user", organization: "org", team: "team", user: "user", token: "oidc-token",
			wantError: "only one of team or user may be specified for token exchange",
		},
		{
			name: "invalid expiration", organization: "org", token: "oidc-token", expiration: "invalid",
			wantError: `could not parse expiration duration: time: invalid duration "invalid"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewAuthContextForTokenExchange(
				tt.organization, tt.team, tt.user, tt.token, tt.expiration, tt.accessToken,
			)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, AuthContext{
				GrantType: AuthContextGrantTypeTokenExchange, Organization: tt.organization,
				Scope: tt.wantScope, Token: tt.token, Expiration: tt.wantDuration,
			}, got)
		})
	}
}
