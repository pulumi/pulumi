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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInsightsResource(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed resource", func(t *testing.T) {
		t.Parallel()

		want := apitype.InsightsResourceWithVersion{
			Account:     "prod-aws",
			Type:        "aws:s3/bucket:Bucket",
			ID:          "my-bucket",
			Version:     7,
			Modified:    time.Date(2026, 5, 1, 14, 30, 0, 0, time.UTC),
			State:       json.RawMessage(`{"arn":"arn:aws:s3:::my-bucket"}`),
			PolicyState: "compliant",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		got, err := client.GetInsightsResource(t.Context(), "acme", "prod-aws", "aws:s3/bucket:Bucket::my-bucket")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("double-encodes accountName and resourceTypeAndId", func(t *testing.T) {
		t.Parallel()

		// The service double-decodes these path parameters, so the wire form
		// must be double-encoded. `/` becomes `%2F` once, then `%252F`. The
		// percent sign of the first encoding (`%`) is what becomes `%25` on
		// the second pass.
		var capturedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// r.URL.Path runs the request through one layer of decoding, so
			// we read RequestURI to see the raw bytes the client sent.
			capturedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"account":"x","type":"y","id":"z","version":1,"modified":"2026-01-01T00:00:00Z"}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsResource(t.Context(),
			"acme", "team/a", "aws:s3/bucket:Bucket::my-bucket")
		require.NoError(t, err)

		// orgName is single-encoded ("acme" has no special chars to encode);
		// accountName and resourceTypeAndId are double-encoded.
		assert.Equal(t,
			"/api/preview/insights/acme/accounts/team%252Fa/resources/aws:s3%252Fbucket:Bucket::my-bucket",
			capturedPath,
		)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsResource(t.Context(),
			"acme", "prod-aws", "aws:s3/bucket:Bucket::missing")
		require.Error(t, err)
	})
}

func TestSearchInsightsResources(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		truthy := true
		want := apitype.InsightsResourceSearchResponse{
			Total: 2,
			Resources: []apitype.InsightsResourceSearchResult{
				{
					Account:  "prod-aws",
					Type:     "aws:s3/bucket:Bucket",
					ID:       "my-bucket",
					URN:      "urn:pulumi:prod::api::aws:s3/bucket:Bucket::my-bucket",
					Stack:    "prod",
					Project:  "api",
					Modified: "2026-05-01T14:30:00Z",
					Managed:  "managed",
					Custom:   &truthy,
				},
				{
					Account: "prod-aws",
					Type:    "aws:s3/bucket:Bucket",
					ID:      "other-bucket",
				},
			},
			Pagination: &apitype.InsightsResourceSearchPagination{
				Cursor: "bookmark",
				Next:   "/api/orgs/acme/search/resourcesv2?cursor=next-token",
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		got, err := client.SearchInsightsResources(t.Context(), "acme",
			apitype.InsightsResourceSearchParams{Query: "type:aws:s3"})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("encodes query parameters", func(t *testing.T) {
		t.Parallel()

		var (
			capturedPath  string
			capturedQuery url.Values
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.SearchInsightsResources(t.Context(), "acme",
			apitype.InsightsResourceSearchParams{
				Query:      "type:aws:s3",
				Sort:       []string{"modified", "name"},
				Ascending:  true,
				Page:       2,
				Size:       50,
				Properties: true,
				Collapse:   true,
			})
		require.NoError(t, err)

		assert.Equal(t, "/api/orgs/acme/search/resourcesv2", capturedPath)
		assert.Equal(t, "type:aws:s3", capturedQuery.Get("query"))
		assert.Equal(t, []string{"modified", "name"}, capturedQuery["sort"])
		assert.Equal(t, "true", capturedQuery.Get("asc"))
		assert.Equal(t, "2", capturedQuery.Get("page"))
		assert.Equal(t, "50", capturedQuery.Get("size"))
		assert.Equal(t, "true", capturedQuery.Get("properties"))
		assert.Equal(t, "true", capturedQuery.Get("collapse"))
	})

	t.Run("omits zero-valued parameters", func(t *testing.T) {
		t.Parallel()

		// Default-valued bool/int/string fields must not appear on the wire,
		// so the server's defaults take effect.
		var capturedQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.SearchInsightsResources(t.Context(), "acme",
			apitype.InsightsResourceSearchParams{})
		require.NoError(t, err)

		assert.Empty(t, capturedQuery)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte("subscription required"))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.SearchInsightsResources(t.Context(), "acme",
			apitype.InsightsResourceSearchParams{Properties: true})
		require.Error(t, err)
		// The 402 status code and the server's body message both make it
		// through the apitype.ErrorResponse formatter ("[code] message").
		assert.Contains(t, err.Error(), "402")
		assert.Contains(t, err.Error(), "subscription required")
	})
}

func TestListInsightsAccounts(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		finished := time.Date(2026, 5, 12, 16, 7, 24, 0, time.UTC)
		want := apitype.ListInsightsAccountsResponse{
			Accounts: []apitype.InsightsAccount{
				{
					ID:                   "79440e1c-089f-4a02-9ad7-0b7effb971b5",
					Name:                 "prod-aws",
					Provider:             "aws",
					ProviderEnvRef:       "team/prod-aws@4",
					ScheduledScanEnabled: true,
					OwnedBy: apitype.InsightsAccountOwner{
						Name:        "Ada Lovelace",
						GitHubLogin: "ada-pulumi-corp",
						AvatarURL:   "https://api.pulumi.com/static/avatars/A.png",
					},
					ScanStatus: &apitype.InsightsAccountScanStatus{
						ID:            "scan-1",
						OrgID:         "org-1",
						UserID:        "user-1",
						Status:        "succeeded",
						StartedAt:     time.Date(2026, 5, 12, 16, 6, 1, 0, time.UTC),
						FinishedAt:    &finished,
						LastUpdatedAt: time.Date(2026, 5, 12, 16, 7, 24, 0, time.UTC),
						ResourceCount: 42,
					},
				},
			},
			NextToken: "next-page",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		got, err := client.ListInsightsAccounts(t.Context(), "acme",
			apitype.ListInsightsAccountsParams{})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("encodes query parameters", func(t *testing.T) {
		t.Parallel()

		var (
			capturedPath  string
			capturedQuery url.Values
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accounts":[]}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.ListInsightsAccounts(t.Context(), "acme",
			apitype.ListInsightsAccountsParams{
				ContinuationToken: "cursor-token",
				Count:             250,
				Parent:            "org-root",
				RoleID:            "role-id",
			})
		require.NoError(t, err)

		assert.Equal(t, "/api/preview/insights/acme/accounts", capturedPath)
		assert.Equal(t, "cursor-token", capturedQuery.Get("continuationToken"))
		assert.Equal(t, "250", capturedQuery.Get("count"))
		assert.Equal(t, "org-root", capturedQuery.Get("parent"))
		assert.Equal(t, "role-id", capturedQuery.Get("roleID"))
	})

	t.Run("omits zero-valued parameters", func(t *testing.T) {
		t.Parallel()

		var capturedQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accounts":[]}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.ListInsightsAccounts(t.Context(), "acme",
			apitype.ListInsightsAccountsParams{})
		require.NoError(t, err)
		assert.Empty(t, capturedQuery)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.ListInsightsAccounts(t.Context(), "acme",
			apitype.ListInsightsAccountsParams{})
		require.Error(t, err)
	})
}

func TestGetInsightsScanLogs(t *testing.T) {
	t.Parallel()

	t.Run("returns continuation-token response", func(t *testing.T) {
		t.Parallel()

		want := apitype.InsightsScanLogs{
			Type: "continuation",
			Lines: []apitype.InsightsScanLogLine{
				{
					Header:    "scan",
					Timestamp: time.Date(2026, 5, 1, 14, 30, 0, 0, time.UTC),
					Line:      "starting scan",
				},
				{
					Timestamp: time.Date(2026, 5, 1, 14, 30, 1, 0, time.UTC),
					Line:      "finished scan",
				},
			},
			ContinuationToken: "next-page",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		got, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "scan-123", apitype.InsightsScanLogsParams{Count: 50})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns step response", func(t *testing.T) {
		t.Parallel()

		want := apitype.InsightsScanLogs{
			Type:       "step",
			Output:     "scan output text\n",
			NextOffset: 1024,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		client := newMockClient(server)
		job, step := 0, 0
		offset := int64(0)
		got, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "scan-123",
			apitype.InsightsScanLogsParams{Job: &job, Step: &step, Offset: &offset})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("encodes query parameters", func(t *testing.T) {
		t.Parallel()

		var (
			capturedPath  string
			capturedQuery url.Values
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.EscapedPath()
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		job, step := 2, 3
		offset := int64(4096)
		_, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "scan-123",
			apitype.InsightsScanLogsParams{
				Job:    &job,
				Step:   &step,
				Offset: &offset,
			})
		require.NoError(t, err)

		assert.Equal(t,
			"/api/preview/insights/acme/accounts/prod-aws/scans/scan-123/logs",
			capturedPath,
		)
		assert.Equal(t, "2", capturedQuery.Get("job"))
		assert.Equal(t, "3", capturedQuery.Get("step"))
		assert.Equal(t, "4096", capturedQuery.Get("offset"))
	})

	t.Run("encodes continuation-mode parameters", func(t *testing.T) {
		t.Parallel()

		var capturedQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "scan-123",
			apitype.InsightsScanLogsParams{
				ContinuationToken: "cursor",
				Count:             100,
			})
		require.NoError(t, err)

		assert.Equal(t, "cursor", capturedQuery.Get("continuationToken"))
		assert.Equal(t, "100", capturedQuery.Get("count"))
		assert.False(t, capturedQuery.Has("job"))
		assert.False(t, capturedQuery.Has("step"))
		assert.False(t, capturedQuery.Has("offset"))
	})

	t.Run("omits zero-valued parameters", func(t *testing.T) {
		t.Parallel()

		// Empty query string lets the server apply its own defaults.
		var capturedQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "scan-123", apitype.InsightsScanLogsParams{})
		require.NoError(t, err)
		assert.Empty(t, capturedQuery)
	})

	t.Run("double-encodes accountName in path", func(t *testing.T) {
		t.Parallel()

		// `/` becomes `%2F` once, then `%252F` for the server-side double-decode.
		var capturedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "team/a", "scan-123", apitype.InsightsScanLogsParams{})
		require.NoError(t, err)

		assert.Equal(t,
			"/api/preview/insights/acme/accounts/team%252Fa/scans/scan-123/logs",
			capturedPath,
		)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer server.Close()

		client := newMockClient(server)
		_, err := client.GetInsightsScanLogs(t.Context(),
			"acme", "prod-aws", "missing-scan", apitype.InsightsScanLogsParams{})
		require.Error(t, err)
	})
}
