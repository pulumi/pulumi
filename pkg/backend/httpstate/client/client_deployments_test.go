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

package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListStackDeployments(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "acme",
		Project: "web",
		Stack:   tokens.MustParseStackName("prod"),
	}

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		want := apitype.ListDeploymentResponseV2{
			ItemsPerPage: 10,
			Total:        2,
			Deployments: []apitype.ListDeploymentSnapshot{
				{
					ID:              "dep-1",
					Created:         "2026-05-01T12:00:00Z",
					Modified:        "2026-05-01T12:05:00Z",
					Status:          "succeeded",
					Version:         42,
					RequestedBy:     apitype.UserInfo{Name: "Alice", GitHubLogin: "alice", AvatarURL: "https://example.com/a.png"},
					ProjectName:     "web",
					StackName:       "prod",
					PulumiOperation: apitype.Update,
					Updates:         []apitype.DeploymentNestedUpdate{},
					Jobs: []apitype.DeploymentJob{{
						Status:      "succeeded",
						Started:     time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
						LastUpdated: time.Date(2026, 5, 1, 12, 5, 0, 0, time.UTC),
						Steps: []apitype.DeploymentStepRun{
							{Name: "pulumi-up", Status: "succeeded"},
						},
					}},
					Initiator: "cli",
				},
				{
					ID:              "dep-2",
					Created:         "2026-04-30T08:00:00Z",
					Modified:        "2026-04-30T08:02:00Z",
					Status:          "failed",
					Version:         41,
					RequestedBy:     apitype.UserInfo{Name: "Bob", GitHubLogin: "bob", AvatarURL: ""},
					PulumiOperation: apitype.Preview,
					Updates:         []apitype.DeploymentNestedUpdate{},
					Jobs:            []apitype.DeploymentJob{},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		c := newMockClient(server)
		got, err := c.ListStackDeployments(t.Context(), stackID, ListStackDeploymentsOptions{})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("passes pagination, sort, asc as query string", func(t *testing.T) {
		t.Parallel()

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"deployments":[],"itemsPerPage":25,"total":0}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListStackDeployments(t.Context(), stackID, ListStackDeploymentsOptions{
			Page:     3,
			PageSize: 25,
			Sort:     "created",
			Asc:      true,
		})
		require.NoError(t, err)

		assert.Equal(t,
			"/api/stacks/acme/web/prod/deployments?asc=true&page=3&pageSize=25&sort=created",
			capturedURI,
		)
	})

	t.Run("omits zero-valued query params", func(t *testing.T) {
		t.Parallel()

		// Zero / empty values mean "use the server default": we should not
		// send them in the URL at all. This keeps requests minimal and
		// avoids accidentally pinning ourselves to a default the server
		// might change.
		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"deployments":[],"itemsPerPage":10,"total":0}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListStackDeployments(t.Context(), stackID, ListStackDeploymentsOptions{})
		require.NoError(t, err)
		assert.Equal(t, "/api/stacks/acme/web/prod/deployments", capturedURI)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid query parameter"))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListStackDeployments(t.Context(), stackID, ListStackDeploymentsOptions{Page: -1})
		require.Error(t, err)
	})
}

func TestPatchStackDeploymentSettings(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "acme",
		Project: "web",
		Stack:   tokens.MustParseStackName("prod"),
	}

	t.Run("posts patch to settings endpoint", func(t *testing.T) {
		t.Parallel()

		var (
			capturedMethod string
			capturedURI    string
			capturedBody   apitype.DeploymentSettings
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedURI = r.URL.RequestURI()
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &capturedBody))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		c := newMockClient(server)
		patch := &apitype.DeploymentSettings{
			SourceContext: &apitype.SourceContext{
				Git: &apitype.SourceContextGit{Branch: "feature"},
			},
		}
		require.NoError(t, c.PatchStackDeploymentSettings(t.Context(), stackID, patch))

		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/api/stacks/acme/web/prod/deployments/settings", capturedURI)
		require.NotNil(t, capturedBody.SourceContext)
		require.NotNil(t, capturedBody.SourceContext.Git)
		assert.Equal(t, "feature", capturedBody.SourceContext.Git.Branch)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid patch"))
		}))
		defer server.Close()

		c := newMockClient(server)
		err := c.PatchStackDeploymentSettings(t.Context(), stackID, &apitype.DeploymentSettings{})
		require.Error(t, err)
	})
}

func TestCancelStackDeployment(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "acme",
		Project: "web",
		Stack:   tokens.MustParseStackName("prod"),
	}

	t.Run("issues POST against the expected path", func(t *testing.T) {
		t.Parallel()

		var (
			capturedMethod string
			capturedURI    string
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedURI = r.URL.RequestURI()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newMockClient(server)
		err := c.CancelStackDeployment(t.Context(), stackID, "dep-123")
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/api/stacks/acme/web/prod/deployments/dep-123/cancel", capturedURI)
	})

	t.Run("propagates 404 errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("deployment not found"))
		}))
		defer server.Close()

		c := newMockClient(server)
		err := c.CancelStackDeployment(t.Context(), stackID, "missing")
		require.Error(t, err)
	})
}
