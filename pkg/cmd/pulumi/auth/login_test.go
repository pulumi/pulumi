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
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginURLResolution tests the complete URL resolution logic in the login command,
// including command-line arguments, flags, environment variables, project settings,
// stored credentials, and OIDC token defaults.
func TestLoginURLResolution(t *testing.T) {
	type capturedLogin struct {
		url       string
		oidcToken string
	}

	credsF := func() (workspace.Credentials, error) {
		return workspace.Credentials{Current: "https://stored-creds.example.com"}, nil
	}

	tests := []struct {
		name          string
		args          []string
		flags         map[string]string
		ws            pkgWorkspace.Context
		envVars       map[string]string
		expectedURL   string
		expectError   bool
		expectedError string
	}{
		{
			name:        "command argument takes precedence",
			args:        []string{"s3://my-bucket"},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "s3://my-bucket",
		},
		{
			name: "cloud-url flag takes precedence",
			flags: map[string]string{
				"cloud-url": "https://custom.example.com",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "https://custom.example.com",
		},
		{
			name: "local flag sets file:// backend",
			flags: map[string]string{
				"local": "true",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "file://~",
		},
		{
			name: "local flag conflicts with cloud-url",
			flags: map[string]string{
				"local":     "true",
				"cloud-url": "https://example.com",
			},
			ws:            &pkgWorkspace.MockContext{},
			expectError:   true,
			expectedError: "a URL may not be specified when --local mode is enabled",
		},
		{
			name: "argument and cloud-url flag conflict",
			args: []string{"s3://bucket"},
			flags: map[string]string{
				"cloud-url": "https://example.com",
			},
			ws:            &pkgWorkspace.MockContext{},
			expectError:   true,
			expectedError: "only one of --cloud-url or argument URL may be specified",
		},
		{
			name: "environment variable used when no explicit URL",
			ws: &pkgWorkspace.MockContext{
				GetStoredCredentialsF: credsF,
			},
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "https://env-backend.example.com",
			},
			expectedURL: "https://env-backend.example.com",
		},
		{
			name: "project backend URL used when no env var",
			ws: &pkgWorkspace.MockContext{
				ReadProjectF: func() (*workspace.Project, string, error) {
					return &workspace.Project{
						Backend: &workspace.ProjectBackend{URL: "https://project-backend.example.com"},
					}, "", nil
				},
				GetStoredCredentialsF: credsF,
			},
			// Clear PULUMI_BACKEND_URL to ensure project backend is used
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "",
			},
			expectedURL: "https://project-backend.example.com",
		},
		{
			name: "stored credentials used as fallback",
			ws: &pkgWorkspace.MockContext{
				GetStoredCredentialsF: credsF,
			},
			// Clear PULUMI_BACKEND_URL to ensure stored credentials are used
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "",
			},
			expectedURL: "https://stored-creds.example.com",
		},
		{
			name: "OIDC token defaults to api.pulumi.com",
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws: &pkgWorkspace.MockContext{},
			envVars: map[string]string{
				"PULUMI_ACCESS_TOKEN": "",
			},
			expectedURL: "https://api.pulumi.com",
		},
		{
			name: "OIDC token with explicit URL uses explicit URL",
			args: []string{"https://custom.example.com"},
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws: &pkgWorkspace.MockContext{},
			envVars: map[string]string{
				"PULUMI_ACCESS_TOKEN": "",
			},
			expectedURL: "https://custom.example.com",
		},
		{
			name: "OIDC token respects env var over default",
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws: &pkgWorkspace.MockContext{},
			envVars: map[string]string{
				"PULUMI_ACCESS_TOKEN": "",
				"PULUMI_BACKEND_URL":  "https://env-backend.example.com",
			},
			expectedURL: "https://env-backend.example.com",
		},
		{
			name: "empty URL without OIDC triggers interactive (captured as empty)",
			ws:   &pkgWorkspace.MockContext{},
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "",
			},
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			var captured capturedLogin
			mockLoginManager := &backend.MockLoginManager{
				LoginF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					color colors.Colorization,
				) (pkgBackend.Backend, error) {
					captured.url = url
					return &pkgBackend.MockBackend{
						URLF:  func() string { return url },
						NameF: func() string { return "test-backend" },
						CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
							return "test-user", nil, nil, nil
						},
					}, nil
				},
				LoginFromAuthContextF: func(
					ctx context.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					authContext workspace.AuthContext,
				) (pkgBackend.Backend, error) {
					captured.url = url
					captured.oidcToken = authContext.Token
					return &pkgBackend.MockBackend{
						URLF:  func() string { return url },
						NameF: func() string { return "test-backend" },
						CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
							return "test-user", nil, nil, nil
						},
					}, nil
				},
			}

			cmd := NewLoginCmd(tt.ws, mockLoginManager)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetContext(t.Context())

			cmd.SetArgs(tt.args)

			if err := cmd.ParseFlags(tt.args); err != nil {
				require.NoError(t, err, "failed to parse flags")
			}

			for name, value := range tt.flags {
				err := cmd.PersistentFlags().Set(name, value)
				require.NoError(t, err, "failed to set flag %s=%s", name, value)
			}

			err := cmd.Execute()

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedError != "" {
					require.ErrorContains(t, err, tt.expectedError)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedURL, captured.url,
					"login should be called with expected URL")
			}
		})
	}
}

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

func TestExtractOIDCDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		organization string
		team         string
		user         string
		token        string
		wantOrg      string
		wantTeam     string
		wantUser     string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "empty token returns unchanged values",
			organization: "my-org",
			team:         "my-team",
			user:         "",
			token:        "",
			wantOrg:      "my-org",
			wantTeam:     "my-team",
			wantUser:     "",
			wantErr:      false,
		},
		{
			name:         "CLI org used as fallback when JWT has no Pulumi aud",
			organization: "my-org",
			token:        createTestJWT(map[string]any{"aud": "https://example.com"}),
			wantOrg:      "my-org",
			wantErr:      false,
		},
		{
			name:         "org extracted from JWT aud claim",
			organization: "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org"}),
			wantOrg:      "test-org",
			wantErr:      false,
		},
		{
			name:         "org extracted from JWT with additional claims",
			organization: "",
			token: createTestJWT(map[string]any{
				"aud": "urn:pulumi:org:test-org",
				"iss": "https://oidc.example.com",
				"exp": 1234567890,
			}),
			wantOrg: "test-org",
			wantErr: false,
		},
		{
			name:         "no org provided and no aud claim",
			organization: "",
			token:        createTestJWT(map[string]any{"sub": "test"}),
			wantOrg:      "",
			wantErr:      false,
		},
		{
			name:         "no org provided and aud claim is not pulumi format",
			organization: "",
			token:        createTestJWT(map[string]any{"aud": "https://example.com"}),
			wantOrg:      "",
			wantErr:      false,
		},
		{
			name:         "conflict: CLI org differs from JWT org",
			organization: "cli-org",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:jwt-org"}),
			wantErr:      true,
			errContains:  "--oidc-org 'cli-org' conflicts with JWT aud claim organization 'jwt-org'",
		},
		{
			name:         "team extracted from JWT scope claim",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:test-org", "scope": "team:dev-team"}),
			wantOrg:      "test-org",
			wantTeam:     "dev-team",
			wantErr:      false,
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
			wantOrg:      "my-org",
			wantUser:     "my-user",
			wantErr:      false,
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
			wantOrg:      "extracted-org",
			wantTeam:     "extracted-team",
			wantErr:      false,
		},
		{
			name:         "CLI team used as fallback when JWT has no scope",
			organization: "my-org",
			team:         "cli-team",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			wantOrg:      "my-org",
			wantTeam:     "cli-team",
			wantErr:      false,
		},
		{
			name:         "CLI user used as fallback when JWT has no scope",
			organization: "my-org",
			user:         "cli-user",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org:my-org"}),
			wantOrg:      "my-org",
			wantUser:     "cli-user",
			wantErr:      false,
		},
		{
			name:         "org from aud and user from scope",
			organization: "",
			user:         "",
			token: createTestJWT(map[string]any{
				"aud":   "urn:pulumi:org:jwt-org",
				"scope": "user:jwt-user",
			}),
			wantOrg:  "jwt-org",
			wantUser: "jwt-user",
			wantErr:  false,
		},
		{
			name:         "scope with multiple values (space-separated string)",
			organization: "",
			team:         "",
			token: createTestJWT(map[string]any{
				"aud":   "urn:pulumi:org:test-org",
				"scope": "openid team:ci-team profile",
			}),
			wantOrg:  "test-org",
			wantTeam: "ci-team",
			wantErr:  false,
		},
		{
			name:         "malformed aud - too few parts",
			organization: "",
			team:         "",
			token:        createTestJWT(map[string]any{"aud": "urn:pulumi:org"}),
			wantOrg:      "",
			wantErr:      false,
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
			wantOrg:      "",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotOrg, gotTeam, gotUser, err := extractOIDCDefaults(
				tt.organization, tt.team, tt.user, tt.token)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, gotOrg, "organization mismatch")
			assert.Equal(t, tt.wantTeam, gotTeam, "team mismatch")
			assert.Equal(t, tt.wantUser, gotUser, "user mismatch")
		})
	}
}
