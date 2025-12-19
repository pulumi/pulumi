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

package tests

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOrg = "test-org"
	// A sample JWT token (expired and not real) for testing purposes
	//nolint:lll // JWT token is long
	testJWT = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMiwiZXhwIjoxNTE2MjM5MDIyfQ.Placeholder"
	// A JWT with an aud claim containing the org name in Pulumi format
	//nolint:lll // JWT token is long
	testJWTWithAud = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMiwiZXhwIjoxNTE2MjM5MDIyLCJhdWQiOiJ1cm46cHVsdW1pOm9yZzp0ZXN0LW9yZyJ9.Placeholder"
)

func TestOIDCLogin(t *testing.T) {
	t.Parallel()

	t.Run("OrganizationToken", func(t *testing.T) {
		t.Parallel()

		// Setup a mock server to handle OIDC token exchange
		var capturedForm url.Values
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())
				capturedForm = r.Form

				// Verify the token exchange request
				assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", r.FormValue("grant_type"))
				assert.Equal(t, "urn:pulumi:org:"+testOrg, r.FormValue("audience"))
				assert.Equal(t, "urn:pulumi:token-type:access_token:organization", r.FormValue("requested_token_type"))
				assert.Equal(t, "urn:ietf:params:oauth:token-type:id_token", r.FormValue("subject_token_type"))
				assert.Equal(t, testJWT, r.FormValue("subject_token"))
				assert.Equal(t, "7200", r.FormValue("expiration"))

				// Return a mock Pulumi access token
				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-access-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:organization",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				// Reply with mock user info
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				t.Logf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Login with OIDC token for organization
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg)

		// Verify we're logged in by running whoami
		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")

		// Verify the token exchange request was made
		require.NotNil(t, capturedForm)
	})

	t.Run("TeamToken", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify team token request
				assert.Equal(t, "urn:pulumi:token-type:access_token:team", r.FormValue("requested_token_type"))
				assert.Equal(t, "team:test-team", r.FormValue("scope"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-team-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:team",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg,
			"--oidc-team", "test-team")

		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})

	t.Run("UserToken", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify personal/user token request
				assert.Equal(t, "urn:pulumi:token-type:access_token:personal", r.FormValue("requested_token_type"))
				assert.Equal(t, "user:test-user", r.FormValue("scope"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-user-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:personal",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg,
			"--oidc-user", "test-user")

		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})

	t.Run("CustomExpiration", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify custom expiration
				assert.Equal(t, "3600", r.FormValue("expiration"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:organization",
					TokenType:       "Bearer",
					ExpiresIn:       3600,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg,
			"--oidc-expiration", "1h")
	})

	t.Run("TokenFromFile", func(t *testing.T) {
		t.Parallel()

		// Create a temporary file with the JWT token
		tmpDir := t.TempDir()
		tokenFile := filepath.Join(tmpDir, "token.jwt")
		err := os.WriteFile(tokenFile, []byte(testJWT), 0o600)
		require.NoError(t, err)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// The token from file should be used
				assert.Equal(t, testJWT, r.FormValue("subject_token"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:organization",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", "file://"+tokenFile,
			"--oidc-org", testOrg)
	})
}

func TestOIDCLoginErrors(t *testing.T) {
	t.Parallel()

	t.Run("WithPulumiAccessToken", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is set
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=pul-access-token")

		// Should fail when both PULUMI_ACCESS_TOKEN and --oidc-token are specified
		_, stderr := e.RunCommandExpectError("pulumi", "login",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg)

		assert.Contains(t, stderr, "cannot perform token exchange when an access token is set as environment variable")
	})

	t.Run("MissingOrgForOIDC", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Should fail when no org is provided and JWT doesn't have Pulumi aud
		_, stderr := e.RunCommandExpectError("pulumi", "login",
			"--oidc-token", testJWT)

		assert.Contains(t, stderr, "org must be set via --oidc-org or the JWT aud claim")
	})

	t.Run("OrgExtractedFromJWTAudClaim", func(t *testing.T) {
		t.Parallel()

		// Setup a mock server to handle OIDC token exchange
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify the token exchange request has the org extracted from the JWT aud claim
				assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", r.FormValue("grant_type"))
				assert.Equal(t, "urn:pulumi:org:"+testOrg, r.FormValue("audience"))
				assert.Equal(t, testJWTWithAud, r.FormValue("subject_token"))

				// Return a mock Pulumi access token
				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-access-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:organization",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				// Reply with mock user info
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				t.Logf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Login with OIDC token WITHOUT --oidc-org flag - org should be extracted from JWT
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWTWithAud)

		// Verify we're logged in by running whoami
		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})

	t.Run("TeamAndUserMutuallyExclusive", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Should fail when both --team and --user are specified
		_, stderr := e.RunCommandExpectError("pulumi", "login",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg,
			"--oidc-team", "test-team",
			"--oidc-user", "test-user")

		assert.Contains(t, stderr, "only one of team or user may be specified")
	})

	t.Run("OIDCNotSupportedForDIYBackend", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		integration.CreateBasicPulumiRepo(e)

		// Should fail when using OIDC flags with local backend
		_, stderr := e.RunCommandExpectError("pulumi", "login", "--local",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg)

		assert.Contains(t, stderr,
			"oidc-token, oidc-org, oidc-team, oidc-user, and oidc-expiration flags are not supported for this type of backend")
	})

	t.Run("TokenExchangeServerError", func(t *testing.T) {
		t.Parallel()

		// Setup server that returns an error for token exchange
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/oauth/token" {
				w.WriteHeader(http.StatusUnauthorized)
				_, err := w.Write([]byte(`{"error":"invalid_token","error_description":"The OIDC token is invalid or expired"}`))
				require.NoError(t, err)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		// Ensure PULUMI_ACCESS_TOKEN is not set to force OIDC login path
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Should fail with server error message
		_, stderr := e.RunCommandExpectError("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", testOrg)

		assert.Contains(t, stderr, "invalid_token")
	})

	t.Run("TeamExtractedFromJWTScopeClaim", func(t *testing.T) {
		t.Parallel()

		// Create a JWT with team in scope claim
		testJWTWithTeam := createJWTWithClaims(map[string]any{
			"aud":   "urn:pulumi:org:test-org",
			"scope": "team:dev-team",
			"iss":   "https://oidc.example.com",
		})

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify team was extracted from JWT
				assert.Equal(t, "urn:pulumi:org:test-org", r.FormValue("audience"))
				assert.Equal(t, "team:dev-team", r.FormValue("scope"))
				assert.Equal(t, "urn:pulumi:token-type:access_token:team", r.FormValue("requested_token_type"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-team-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:team",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Login with JWT that has team in scope claim - should extract both org and team
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWTWithTeam)

		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})

	t.Run("UserExtractedFromScopeClaim", func(t *testing.T) {
		t.Parallel()

		// Create a JWT with user in scope claim
		testJWTWithUser := createJWTWithClaims(map[string]any{
			"aud":   "urn:pulumi:org:test-org",
			"scope": "user:test-user-123",
		})

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify user was extracted from scope claim
				assert.Equal(t, "urn:pulumi:org:test-org", r.FormValue("audience"))
				assert.Equal(t, "user:test-user-123", r.FormValue("scope"))
				assert.Equal(t, "urn:pulumi:token-type:access_token:personal", r.FormValue("requested_token_type"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-personal-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:personal",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Login with JWT that has user in scope claim - should extract both org and user
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWTWithUser)

		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})

	t.Run("ExplicitFlagsUsedAsFallback", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/oauth/token":
				require.NoError(t, r.ParseForm())

				// Verify explicit flags are used when JWT lacks Pulumi claims
				assert.Equal(t, "urn:pulumi:org:explicit-org", r.FormValue("audience"))
				assert.Equal(t, "team:explicit-team", r.FormValue("scope"))
				assert.Equal(t, testJWT, r.FormValue("subject_token"))

				resp := apitype.TokenExchangeGrantResponse{
					AccessToken:     "pul-test-token",
					IssuedTokenType: "urn:pulumi:token-type:access_token:team",
					TokenType:       "Bearer",
					ExpiresIn:       7200,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				err := json.NewEncoder(w).Encode(resp)
				require.NoError(t, err)
			case "/api/user":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"githubLogin":"test-user","name":"Test User","email":"test@example.com"}`))
				require.NoError(t, err)
			case "/api/capabilities":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		e.Env = append(e.Env, "PULUMI_ACCESS_TOKEN=")

		// Explicit flags used as fallback when JWT lacks Pulumi-specific claims
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--oidc-org", "explicit-org",
			"--oidc-team", "explicit-team")

		stdout, _ := e.RunCommand("pulumi", "whoami")
		assert.Contains(t, stdout, "test-user")
	})
}

// createJWTWithClaims creates a test JWT with the given claims.
// Note: This creates an unsigned JWT for testing purposes only.
func createJWTWithClaims(claims map[string]any) string {
	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	// Use encoding/base64 RawURLEncoding (no padding) as per JWT spec
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return headerEncoded + "." + claimsEncoded + "." + signature
}
