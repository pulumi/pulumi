// Copyright 2020-2024, Pulumi Corporation.
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

package workspace

import (
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestJWT creates a test JWT with the given claims.
// Note: This creates an unsigned JWT for testing purposes only.
func createTestJWT(claims map[string]any) string {
	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create a fake signature (not cryptographically valid, but parseable)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return headerEncoded + "." + claimsEncoded + "." + signature
}

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	// save and remember to restore creds in ~/.pulumi/credentials
	// as the test will be modifying them
	oldCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	defer func() {
		err := StoreCredentials(oldCreds)
		require.NoError(t, err)
	}()

	// use test creds that have at least 1 AccessToken to force a
	// disk write and contention
	testCreds := Credentials{
		AccessTokens: map[string]string{
			"token-name": "token-value",
		},
	}

	// using 1000 may trigger sporadic 'Too many open files'
	n := 256

	wg := &sync.WaitGroup{}
	wg.Add(2 * n)

	// Store testCreds initially so asserts in
	// GetStoredCredentials goroutines find the expected data
	err = StoreCredentials(testCreds)
	require.NoError(t, err)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := StoreCredentials(testCreds)
			require.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			creds, err := GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, "token-value", creds.AccessTokens["token-name"])
		}()
	}
	wg.Wait()
}

func TestNewAuthContextForTokenExchange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		organization string
		team         string
		user         string
		token        string
		expiration   string
		wantErr      bool
		wantOrg      string
		wantTeam     string
		wantUser     string
		errContains  string
	}{
		{
			name:         "CLI org used as fallback when JWT has no Pulumi aud",
			organization: "my-org",
			token:        createTestJWT(map[string]any{"aud": "https://example.com"}),
			wantErr:      false,
			wantOrg:      "my-org",
		},
		{
			name:         "org extracted from JWT aud claim",
			organization: "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org"}),
			wantErr:      false,
			wantOrg:      "test-org",
		},
		{
			name:         "org extracted from JWT with additional claims",
			organization: "",
			token: createTestJWT(map[string]any{
				"aud": "urn:pulumi:org:test-org",
				"iss": "https://oidc.example.com",
				"exp": 1234567890,
			}),
			wantErr: false,
			wantOrg: "test-org",
		},
		{
			name:         "no org provided and no aud claim",
			organization: "",
			token:        createTestJWT(map[string]any{"sub": "test"}),
			wantErr:      true,
			errContains:  "org must be set",
		},
		{
			name:         "no org provided and aud claim is not pulumi format",
			organization: "",
			token:        createTestJWT(map[string]any{"aud": "https://example.com"}),
			wantErr:      true,
			errContains:  "org must be set",
		},
		{
			name:         "conflict: CLI org differs from JWT org",
			organization: "cli-org",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:jwt-org"}),
			wantErr:      true,
			errContains:  "--oidc-org 'cli-org' conflicts with JWT aud claim organization 'jwt-org'",
		},
		{
			name:         "empty token",
			organization: "my-org",
			token:        "",
			wantErr:      true,
			errContains:  "oidc token must be specified",
		},
		{
			name:         "both team and user specified",
			organization: "my-org",
			team:         "my-team",
			user:         "my-user",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			wantErr:      true,
			errContains:  "only one of team or user",
		},
		{
			name:         "invalid expiration duration",
			organization: "my-org",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			expiration:   "invalid",
			wantErr:      true,
			errContains:  "could not parse expiration duration",
		},
		{
			name:         "team extracted from JWT scope claim",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org", "scope": "team:dev-team"}),
			wantErr:      false,
			wantOrg:      "test-org",
			wantTeam:     "dev-team",
		},
		{
			name:         "conflict: CLI team differs from JWT scope team",
			organization: "",
			team:         "cli-team",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org", "scope": "team:jwt-team"}),
			wantErr:      true,
			errContains:  "--oidc-team 'cli-team' conflicts with JWT scope team 'jwt-team'",
		},
		{
			name:         "user extracted from JWT scope claim",
			organization: "my-org",
			user:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org", "scope": "user:my-user"}),
			wantErr:      false,
			wantOrg:      "my-org",
			wantUser:     "my-user",
		},
		{
			name:         "conflict: CLI user differs from JWT scope user",
			organization: "my-org",
			user:         "cli-user",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org", "scope": "user:jwt-user"}),
			wantErr:      true,
			errContains:  "conflicts with JWT scope user",
		},
		{
			name:         "org and team both extracted from JWT",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:extracted-org", "scope": "team:extracted-team"}),
			wantErr:      false,
			wantOrg:      "extracted-org",
			wantTeam:     "extracted-team",
		},
		{
			name:         "CLI team used as fallback when JWT has no scope",
			organization: "my-org",
			team:         "cli-team",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			wantErr:      false,
			wantOrg:      "my-org",
			wantTeam:     "cli-team",
		},
		{
			name:         "CLI user used as fallback when JWT has no scope",
			organization: "my-org",
			user:         "cli-user",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			wantErr:      false,
			wantOrg:      "my-org",
			wantUser:     "cli-user",
		},
		{
			name:         "org from aud and user from scope",
			organization: "",
			user:         "",
			token: createTestJWT(map[string]any{
				"aud":   "urn:pulumi:org:jwt-org",
				"scope": "user:jwt-user",
			}),
			wantErr:  false,
			wantOrg:  "jwt-org",
			wantUser: "jwt-user",
		},
		{
			name:         "scope with multiple values (space-separated string)",
			organization: "",
			team:         "",
			token: createTestJWT(map[string]any{
				"aud":   "urn:pulumi:org:test-org",
				"scope": "openid team:ci-team profile",
			}),
			wantErr:  false,
			wantOrg:  "test-org",
			wantTeam: "ci-team",
		},
		{
			name:         "malformed aud - too few parts",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org"}),
			wantErr:      true,
			errContains:  "org must be set",
		},
		{
			name:         "JWT with both team and user in scope is invalid",
			organization: "",
			team:         "",
			user:         "",
			token: createTestJWT(map[string]any{
				"aud": "urn:pulumi:org:test-org", "scope": "team:dev-team user:test-user",
			}),
			wantErr:     true,
			errContains: "JWT scope contains both team",
		},
		{
			name:         "aud with extra parts is ignored",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org:extra:parts"}),
			wantErr:      true,
			errContains:  "org must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authContext, err := NewAuthContextForTokenExchange(
				tt.organization, tt.team, tt.user, tt.token, tt.expiration)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, authContext.Organization)
			assert.Equal(t, AuthContextGrantTypeTokenExchange, authContext.GrantType)
			assert.Equal(t, tt.token, authContext.Token)

			// Determine expected team - explicit takes precedence, then JWT extraction
			expectedTeam := tt.team
			if expectedTeam == "" && tt.wantTeam != "" {
				expectedTeam = tt.wantTeam
			}

			// Determine expected user - explicit takes precedence, then JWT extraction
			expectedUser := tt.user
			if expectedUser == "" && tt.wantUser != "" {
				expectedUser = tt.wantUser
			}

			if expectedTeam != "" {
				assert.Equal(t, "team:"+expectedTeam, authContext.Scope)
			} else if expectedUser != "" {
				assert.Equal(t, "user:"+expectedUser, authContext.Scope)
			} else {
				assert.Equal(t, "", authContext.Scope)
			}
		})
	}
}

func TestNewAuthContextForTokenExchange_WithAccessTokenInEnvironment(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "existing-token")

	token := createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"})
	_, err := NewAuthContextForTokenExchange("my-org", "", "", token, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot perform token exchange when an access token is set")
}
