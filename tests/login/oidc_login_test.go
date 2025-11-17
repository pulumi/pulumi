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
	// A sample JWT token (expired and not real) for testing purposes
	//nolint:lll // JWT token is long
	testJWT = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IlRlc3QgVXNlciIsImlhdCI6MTUxNjIzOTAyMiwiZXhwIjoxNTE2MjM5MDIyfQ.Placeholder"
	testOrg = "test-org"
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

		// Login with OIDC token for organization
		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--org", testOrg)

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

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--org", testOrg,
			"--team", "test-team")

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

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--org", testOrg,
			"--user", "test-user")

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

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--org", testOrg,
			"--expiration", "3600")
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

		e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", "file://"+tokenFile,
			"--org", testOrg)
	})
}

func TestOIDCLoginErrors(t *testing.T) {
	t.Parallel()

	t.Run("TeamAndUserMutuallyExclusive", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		// Should fail when both --team and --user are specified
		_, stderr := e.RunCommandExpectError("pulumi", "login",
			"--oidc-token", testJWT,
			"--org", testOrg,
			"--team", "test-team",
			"--user", "test-user")

		assert.Contains(t, stderr, "only one of --team or --user may be specified")
	})

	t.Run("OIDCNotSupportedForDIYBackend", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)

		// Should fail when using OIDC flags with local backend
		_, stderr := e.RunCommandExpectError("pulumi", "login", "--local",
			"--oidc-token", testJWT,
			"--org", testOrg)

		assert.Contains(t, stderr,
			"oidc-token, org, team, user, and expiration flags are not supported for this type of backend")
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

		// Should fail with server error message
		_, stderr := e.RunCommandExpectError("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT,
			"--org", testOrg)

		assert.Contains(t, stderr, "invalid_token")
	})

	t.Run("MissingOrgForOIDC", func(t *testing.T) {
		t.Parallel()

		// Note: Based on the code, --org is not validated as required in the CLI itself,
		// but the server will likely reject it. This test documents current behavior.
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/oauth/token" {
				require.NoError(t, r.ParseForm())
				// Verify that audience is sent (even if empty)
				assert.Equal(t, "urn:pulumi:org:", r.FormValue("audience"))

				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte(`{"error":"invalid_request","error_description":"organization is required"}`))
				require.NoError(t, err)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		_, stderr := e.RunCommandExpectError("pulumi", "login", "--cloud-url", server.URL, "--insecure",
			"--oidc-token", testJWT)

		assert.Contains(t, stderr, "invalid_request")
	})
}
