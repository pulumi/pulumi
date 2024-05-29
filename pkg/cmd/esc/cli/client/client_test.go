// Copyright 2023, Pulumi Corporation.

package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, method, path string, handler func(w http.ResponseWriter, r *http.Request)) *client {
	mux, client := newTestServer(t)
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, method, r.Method)
		handler(w, r)
	})
	return client
}

func newTestServer(t *testing.T) (*http.ServeMux, *client) {
	const userAgent = "test-user-agent"
	const token = "test-token"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		assert.FailNow(t, "unexpected %v %v", r.Method, r.URL)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, userAgent, r.Header.Get("User-Agent"))
		require.Equal(t, "token "+token, r.Header.Get("Authorization"))
		require.Equal(t, "application/vnd.pulumi+8", r.Header.Get("Accept"))

		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	client := newClient(userAgent, server.URL, token, server.Client())
	return mux, client
}

func TestGetPulumiAccountDetails(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		expected := serviceUser{
			GitHubLogin: "test-user",
			Organizations: []serviceUserInfo{{
				Name:        "test-org",
				GitHubLogin: "test-org",
			}},
			TokenInfo: &serviceTokenInfo{
				Name: "my-token",
			},
		}

		client := newTestClient(t, http.MethodGet, "/api/user", func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(expected)
			require.NoError(t, err)
		})

		user, orgs, info, err := client.GetPulumiAccountDetails(context.Background())
		require.NoError(t, err)

		assert.Equal(t, expected.GitHubLogin, user)
		assert.Equal(t, []string{"test-org"}, orgs)
		assert.Equal(t, &workspace.TokenInformation{Name: expected.TokenInfo.Name}, info)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		client := newTestClient(t, http.MethodGet, "/api/user", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    401,
				Message: "unauthorized",
			})
			require.NoError(t, err)
		})

		_, _, _, err := client.GetPulumiAccountDetails(context.Background())
		assert.ErrorContains(t, err, "unauthorized")
	})
}

func TestListEnvironments(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		expected := []OrgEnvironment{
			{Organization: "org-1", Name: "env-1"},
			{Organization: "org-1", Name: "env-2"},
			{Organization: "org-2", Name: "env-1"},
		}

		expectedToken := "next-token"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "", r.URL.Query().Get("continuationToken"))
			assert.Equal(t, "", r.URL.Query().Get("organization"))

			err := json.NewEncoder(w).Encode(ListEnvironmentsResponse{
				Environments: expected,
				NextToken:    expectedToken,
			})
			require.NoError(t, err)
		})

		actual, token, err := client.ListEnvironments(context.Background(), "", "")
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
		assert.Equal(t, expectedToken, token)
	})

	t.Run("org filter", func(t *testing.T) {
		org := "org-1"
		expected := []OrgEnvironment{
			{Organization: org, Name: "env-1"},
			{Organization: org, Name: "env-2"},
		}

		expectedToken := "next-token"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "", r.URL.Query().Get("continuationToken"))
			assert.Equal(t, org, r.URL.Query().Get("organization"))

			err := json.NewEncoder(w).Encode(ListEnvironmentsResponse{
				Environments: expected,
				NextToken:    expectedToken,
			})
			require.NoError(t, err)
		})

		actual, token, err := client.ListEnvironments(context.Background(), org, "")
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
		assert.Equal(t, expectedToken, token)
	})

	t.Run("token", func(t *testing.T) {
		token := "next-token"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, token, r.URL.Query().Get("continuationToken"))
			assert.Equal(t, "", r.URL.Query().Get("organization"))

			err := json.NewEncoder(w).Encode(ListEnvironmentsResponse{})
			require.NoError(t, err)
		})

		actual, token, err := client.ListEnvironments(context.Background(), "", token)
		require.NoError(t, err)
		assert.Nil(t, actual)
		assert.Equal(t, "", token)
	})
}

func TestCreateEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := client.CreateEnvironment(context.Background(), "test-org", "test-env")
		assert.NoError(t, err)
	})

	t.Run("Conflict", func(t *testing.T) {
		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    409,
				Message: "conflict",
			})
			require.NoError(t, err)
		})
		err := client.CreateEnvironment(context.Background(), "test-org", "test-env")
		assert.ErrorContains(t, err, "conflict")
	})

}

func TestGetEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		expectedYAML := []byte("arbitrary content")
		expectedTag := "new-tag"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", expectedTag)
			w.Header().Set(revisionHeader, "1")
			_, err := w.Write(expectedYAML)
			require.NoError(t, err)
		})

		actualYAML, actualTag, revision, err := client.GetEnvironment(context.Background(), "test-org", "test-env", "", false)
		require.NoError(t, err)
		assert.Equal(t, string(expectedYAML), string(actualYAML))
		assert.Equal(t, expectedTag, actualTag)
		assert.Equal(t, 1, revision)
	})

	t.Run("Revision", func(t *testing.T) {
		expectedYAML := []byte("arbitrary content")
		expectedTag := "new-tag"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/versions/42", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", expectedTag)
			w.Header().Set(revisionHeader, "1")
			_, err := w.Write(expectedYAML)
			require.NoError(t, err)
		})

		actualYAML, actualTag, revision, err := client.GetEnvironment(context.Background(), "test-org", "test-env", "42", false)
		require.NoError(t, err)
		assert.Equal(t, string(expectedYAML), string(actualYAML))
		assert.Equal(t, expectedTag, actualTag)
		assert.Equal(t, 1, revision)
	})

	t.Run("Tag", func(t *testing.T) {
		expectedYAML := []byte("arbitrary content")
		expectedTag := "new-tag"

		mux, client := newTestServer(t)

		mux.HandleFunc("/api/preview/environments/test-org/test-env/versions/stable", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", expectedTag)
			w.Header().Set(revisionHeader, "1")
			_, err := w.Write(expectedYAML)
			require.NoError(t, err)
		})

		actualYAML, actualTag, revision, err := client.GetEnvironment(context.Background(), "test-org", "test-env", "stable", false)
		require.NoError(t, err)
		assert.Equal(t, string(expectedYAML), string(actualYAML))
		assert.Equal(t, expectedTag, actualTag)
		assert.Equal(t, 1, revision)
	})

	t.Run("Decrypt", func(t *testing.T) {
		expectedYAML := []byte("arbitrary content")
		expectedTag := "new-tag"

		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/decrypt", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", expectedTag)
			w.Header().Set(revisionHeader, "1")
			_, err := w.Write(expectedYAML)
			require.NoError(t, err)
		})

		actualYAML, actualTag, revision, err := client.GetEnvironment(context.Background(), "test-org", "test-env", "", true)
		require.NoError(t, err)
		assert.Equal(t, string(expectedYAML), string(actualYAML))
		assert.Equal(t, expectedTag, actualTag)
		assert.Equal(t, 1, revision)
	})

	t.Run("Not found", func(t *testing.T) {
		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    404,
				Message: "not found",
			})
			require.NoError(t, err)
		})

		_, _, _, err := client.GetEnvironment(context.Background(), "test-org", "test-env", "", false)
		assert.ErrorContains(t, err, "not found")
	})
}

func TestUpdateEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		yaml := []byte("new definition")
		tag := "old tag"

		client := newTestClient(t, http.MethodPatch, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, tag, r.Header.Get("ETag"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, yaml, body)

			w.Header().Set(revisionHeader, "1")
			w.WriteHeader(http.StatusOK)
		})

		diags, revision, err := client.UpdateEnvironmentWithRevision(context.Background(), "test-org", "test-env", yaml, tag)
		require.NoError(t, err)
		assert.Equal(t, 1, revision)
		assert.Len(t, diags, 0)
	})

	t.Run("Diags", func(t *testing.T) {
		expected := []EnvironmentDiagnostic{
			{
				Range: &esc.Range{
					Environment: "test-env",
					Begin:       esc.Pos{Line: 42, Column: 1},
					End:         esc.Pos{Line: 42, Column: 42},
				},
				Summary: "diag 1",
			},
			{
				Range: &esc.Range{
					Environment: "import-env",
					Begin:       esc.Pos{Line: 1, Column: 2},
					End:         esc.Pos{Line: 3, Column: 4},
				},
				Summary: "diag 2",
			},
		}

		client := newTestClient(t, http.MethodPatch, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)

			err := json.NewEncoder(w).Encode(EnvironmentErrorResponse{
				Code:        400,
				Message:     "bad request",
				Diagnostics: expected,
			})
			require.NoError(t, err)
		})

		diags, revision, err := client.UpdateEnvironmentWithRevision(context.Background(), "test-org", "test-env", nil, "")
		require.Equal(t, 0, revision)
		require.NoError(t, err)
		assert.Equal(t, expected, diags)
	})

	t.Run("Conflict", func(t *testing.T) {
		client := newTestClient(t, http.MethodPatch, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    409,
				Message: "conflict",
			})
			require.NoError(t, err)
		})
		_, revision, err := client.UpdateEnvironmentWithRevision(context.Background(), "test-org", "test-env", nil, "")
		require.Equal(t, 0, revision)
		assert.ErrorContains(t, err, "conflict")
	})
}

func TestDeleteEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		client := newTestClient(t, http.MethodDelete, "/api/preview/environments/test-org/test-env", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := client.DeleteEnvironment(context.Background(), "test-org", "test-env")
		assert.NoError(t, err)
	})
}

func TestOpenEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		const expectedID = "open-id"
		duration := 2 * time.Hour

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env/open", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, duration.String(), r.URL.Query().Get("duration"))

			err := json.NewEncoder(w).Encode(map[string]any{"id": expectedID})
			require.NoError(t, err)
		})

		id, diags, err := client.OpenEnvironment(context.Background(), "test-org", "test-env", "", duration)
		require.NoError(t, err)
		assert.Equal(t, expectedID, id)
		assert.Empty(t, diags)
	})

	t.Run("Revision", func(t *testing.T) {
		const expectedID = "open-id"
		duration := 2 * time.Hour

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env/versions/42/open", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, duration.String(), r.URL.Query().Get("duration"))

			err := json.NewEncoder(w).Encode(map[string]any{"id": expectedID})
			require.NoError(t, err)
		})

		id, diags, err := client.OpenEnvironment(context.Background(), "test-org", "test-env", "42", duration)
		require.NoError(t, err)
		assert.Equal(t, expectedID, id)
		assert.Empty(t, diags)
	})

	t.Run("Tag", func(t *testing.T) {
		const expectedID = "open-id"
		duration := 2 * time.Hour

		mux, client := newTestServer(t)

		mux.HandleFunc("/api/preview/environments/test-org/test-env/versions/stable/open", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, duration.String(), r.URL.Query().Get("duration"))

			err := json.NewEncoder(w).Encode(map[string]any{"id": expectedID})
			require.NoError(t, err)
		})

		id, diags, err := client.OpenEnvironment(context.Background(), "test-org", "test-env", "stable", duration)
		require.NoError(t, err)
		assert.Equal(t, expectedID, id)
		assert.Empty(t, diags)
	})

	t.Run("Diags", func(t *testing.T) {
		expected := []EnvironmentDiagnostic{
			{
				Range: &esc.Range{
					Environment: "test-env",
					Begin:       esc.Pos{Line: 42, Column: 1},
					End:         esc.Pos{Line: 42, Column: 42},
				},
				Summary: "diag 1",
			},
			{
				Range: &esc.Range{
					Environment: "import-env",
					Begin:       esc.Pos{Line: 1, Column: 2},
					End:         esc.Pos{Line: 3, Column: 4},
				},
				Summary: "diag 2",
			},
		}

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env/open", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)

			err := json.NewEncoder(w).Encode(EnvironmentErrorResponse{
				Code:        400,
				Message:     "bad request",
				Diagnostics: expected,
			})
			require.NoError(t, err)
		})

		_, diags, err := client.OpenEnvironment(context.Background(), "test-org", "test-env", "", 2*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, expected, diags)
	})

	t.Run("Not found", func(t *testing.T) {
		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/test-env/open", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    404,
				Message: "not found",
			})
			require.NoError(t, err)
		})

		_, _, err := client.OpenEnvironment(context.Background(), "test-org", "test-env", "", 2*time.Hour)
		assert.ErrorContains(t, err, "not found")
	})
}

func TestCheckYAMLEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		yaml := []byte(`{"values":{"foo":"bar"}}`)

		expected := &esc.Environment{
			Exprs:      map[string]esc.Expr{"foo": {Literal: "bar"}},
			Properties: map[string]esc.Value{"foo": esc.NewValue("bar")},
			Schema:     schema.Record(map[string]schema.Builder{"foo": schema.String().Const("bar")}).Schema(),
		}

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/yaml/check", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, yaml, body)

			err = json.NewEncoder(w).Encode(expected)
			require.NoError(t, err)
		})

		env, diags, err := client.CheckYAMLEnvironment(context.Background(), "test-org", yaml)
		require.NoError(t, err)
		assert.Equal(t, expected, env)
		assert.Empty(t, diags)
	})

	t.Run("Diags", func(t *testing.T) {
		yaml := []byte(`arbitrary`)

		expected := []EnvironmentDiagnostic{
			{
				Range: &esc.Range{
					Environment: "test-env",
					Begin:       esc.Pos{Line: 42, Column: 1},
					End:         esc.Pos{Line: 42, Column: 42},
				},
				Summary: "diag 1",
			},
			{
				Range: &esc.Range{
					Environment: "import-env",
					Begin:       esc.Pos{Line: 1, Column: 2},
					End:         esc.Pos{Line: 3, Column: 4},
				},
				Summary: "diag 2",
			},
		}

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/yaml/check", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)

			err := json.NewEncoder(w).Encode(EnvironmentErrorResponse{
				Code:        400,
				Message:     "bad request",
				Diagnostics: expected,
			})
			require.NoError(t, err)
		})

		_, diags, err := client.CheckYAMLEnvironment(context.Background(), "test-org", yaml)
		require.NoError(t, err)
		assert.Equal(t, expected, diags)
	})
}

func TestOpenYAMLEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		yaml := []byte(`{"values":{"foo":"bar"}}`)

		const expectedID = "open-id"
		duration := 2 * time.Hour

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/yaml/open", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, duration.String(), r.URL.Query().Get("duration"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, yaml, body)

			err = json.NewEncoder(w).Encode(map[string]any{"id": expectedID})
			require.NoError(t, err)
		})

		id, diags, err := client.OpenYAMLEnvironment(context.Background(), "test-org", yaml, duration)
		require.NoError(t, err)
		assert.Equal(t, expectedID, id)
		assert.Empty(t, diags)
	})

	t.Run("Diags", func(t *testing.T) {
		yaml := []byte(`arbitrary`)

		expected := []EnvironmentDiagnostic{
			{
				Range: &esc.Range{
					Environment: "test-env",
					Begin:       esc.Pos{Line: 42, Column: 1},
					End:         esc.Pos{Line: 42, Column: 42},
				},
				Summary: "diag 1",
			},
			{
				Range: &esc.Range{
					Environment: "import-env",
					Begin:       esc.Pos{Line: 1, Column: 2},
					End:         esc.Pos{Line: 3, Column: 4},
				},
				Summary: "diag 2",
			},
		}

		client := newTestClient(t, http.MethodPost, "/api/preview/environments/test-org/yaml/open", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)

			err := json.NewEncoder(w).Encode(EnvironmentErrorResponse{
				Code:        400,
				Message:     "bad request",
				Diagnostics: expected,
			})
			require.NoError(t, err)
		})

		_, diags, err := client.OpenYAMLEnvironment(context.Background(), "test-org", yaml, 2*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, expected, diags)
	})

}

func TestGetOpenEnvironment(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		expected := &esc.Environment{
			Exprs:      map[string]esc.Expr{"foo": {Literal: "bar"}},
			Properties: map[string]esc.Value{"foo": esc.NewValue("bar")},
			Schema:     schema.Record(map[string]schema.Builder{"foo": schema.String().Const("bar")}).Schema(),
		}

		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/open/session", func(w http.ResponseWriter, r *http.Request) {
			err := json.NewEncoder(w).Encode(expected)
			require.NoError(t, err)
		})

		env, err := client.GetOpenEnvironment(context.Background(), "test-org", "test-env", "session")
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Not found", func(t *testing.T) {
		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/open/session", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    404,
				Message: "not found",
			})
			require.NoError(t, err)
		})

		_, err := client.GetOpenEnvironment(context.Background(), "test-org", "test-env", "session")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestGetOpenProperty(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		property := `foo[0].baz["qux"]`
		expected := esc.NewValue("bar")

		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/open/session", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, property, r.URL.Query().Get("property"))

			err := json.NewEncoder(w).Encode(expected)
			require.NoError(t, err)
		})

		val, err := client.GetOpenProperty(context.Background(), "test-org", "test-env", "session", property)
		require.NoError(t, err)
		assert.Equal(t, &expected, val)
	})

	t.Run("Not found", func(t *testing.T) {
		client := newTestClient(t, http.MethodGet, "/api/preview/environments/test-org/test-env/open/session", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			err := json.NewEncoder(w).Encode(apitype.ErrorResponse{
				Code:    404,
				Message: "not found",
			})
			require.NoError(t, err)
		})

		_, err := client.GetOpenProperty(context.Background(), "test-org", "test-env", "session", "foo")
		assert.ErrorContains(t, err, "not found")
	})
}
