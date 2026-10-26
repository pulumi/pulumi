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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
)

// testJWT is a test JWT token used in tests.
//
//nolint:lll // JWT token is long
const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

func TestIsExpectedTokenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		isExpected bool
	}{
		{
			name:       "JWT token",
			token:      testJWT,
			isExpected: true,
		},
		{
			name:       "empty token",
			token:      "",
			isExpected: false,
		},
		{
			name:       "unexpected token",
			token:      "unexpected-token",
			isExpected: false,
		},
		{
			name:       "random string",
			token:      "randomstring123",
			isExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isExpectedTokenFormat(tt.token)
			if tt.isExpected {
				assert.True(t, result)
			} else {
				assert.False(t, result)
			}
		})
	}
}

func TestGetTokenValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       string
		setupFile   func(*testing.T) string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "direct JWT token",
			token:     testJWT,
			wantValue: testJWT,
			wantErr:   false,
		},
		{
			name:  "token from file",
			token: "file://",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "token.txt")
				require.NoError(t, os.WriteFile(path, []byte("  "+testJWT+"  \n"), 0o600))
				return path
			},
			wantValue: testJWT,
			wantErr:   false,
		},
		{
			name:        "token from nonexistent file",
			token:       "file:///nonexistent/path/to/token.txt",
			wantErr:     true,
			errContains: "reading token from file",
		},
		{
			name:  "empty file",
			token: "file://",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "token.txt")
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				return path
			},
			wantErr:     true,
			errContains: "is empty",
		},
		{
			name:  "file with unexpected token format",
			token: "file://",
			setupFile: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "token.txt")
				require.NoError(t, os.WriteFile(path, []byte("unexpected-token-format\n"), 0o600))
				return path
			},
			wantErr:     true,
			errContains: "token format in file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token := tt.token
			if tt.setupFile != nil {
				filePath := tt.setupFile(t)
				token = "file://" + filePath
			}

			value, err := getTokenValue(token)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, value)
			}
		})
	}
}

func TestExchangeOidcToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oidcToken    string
		organization string
		scope        string
		expiration   time.Duration
		setupServer  func() *httptest.Server
		wantErr      bool
		errContains  string
		checkResult  func(*testing.T, string, time.Time)
	}{
		{
			name:         "empty oidc token",
			oidcToken:    "",
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			wantErr:      true,
			errContains:  "Unauthorized: No credentials provided or are invalid",
		},
		{
			name:         "invalid oidc token format",
			oidcToken:    "invalid-token-format",
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			wantErr:      true,
			errContains:  "Failed to read OIDC token",
		},
		{
			name:         "successful token exchange",
			oidcToken:    testJWT,
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/oauth/token" {
						resp := apitype.TokenExchangeGrantResponse{
							AccessToken: "pul-jwt-access-token",
							ExpiresIn:   3600,
							TokenType:   "Bearer",
							Scope:       "org:test-org",
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(resp)
					}
				}))
			},
			wantErr: false,
			checkResult: func(t *testing.T, accessToken string, expiresAt time.Time) {
				assert.Equal(t, "pul-jwt-access-token", accessToken)
				assert.False(t, expiresAt.IsZero())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cloudURL := ""
			if tt.setupServer != nil {
				server := tt.setupServer()
				defer server.Close()
				cloudURL = server.URL
			}

			accessToken, expiresAt, err := exchangeOidcToken(
				t.Context(), diagtest.LogSink(t), cloudURL, false, tt.oidcToken, tt.organization, tt.scope, tt.expiration,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, accessToken, expiresAt)
				}
			}
		})
	}
}
