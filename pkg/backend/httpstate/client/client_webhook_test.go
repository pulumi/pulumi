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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListStackWebhooks(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		format := "raw"
		want := []apitype.Webhook{
			{
				OrganizationName: "my-org",
				Name:             "deploy-hook",
				DisplayName:      "Deploy Hook",
				PayloadURL:       "https://example.com/webhook",
				Active:           true,
				Format:           &format,
				Filters:          []string{"stack_update"},
			},
			{
				OrganizationName: "my-org",
				Name:             "slack-hook",
				DisplayName:      "Slack Notifications",
				PayloadURL:       "https://hooks.slack.com/services/T00/B00/xxx",
				Active:           false,
			},
		}

		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(want) //nolint:gosec // test data
			require.NoError(t, err)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.ListStackWebhooks(t.Context(), stackID)
		require.NoError(t, err)

		assert.Equal(t, "/api/stacks/my-org/my-project/dev/hooks", gotPath)
		assert.Equal(t, want, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusInternalServerError, `{"message":"internal error"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.ListStackWebhooks(t.Context(), stackID)
		assert.ErrorContains(t, err, "internal error")
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.ListStackWebhooks(t.Context(), stackID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestGetStackWebhook(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		format := "raw"
		want := apitype.Webhook{
			OrganizationName: "my-org",
			Name:             "deploy-hook",
			DisplayName:      "Deploy Hook",
			PayloadURL:       "https://example.com/webhook",
			Active:           true,
			Format:           &format,
			Filters:          []string{"stack_update"},
			Groups:           []string{"stacks"},
		}

		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(want)
			require.NoError(t, err)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.GetStackWebhook(t.Context(), stackID, "deploy-hook")
		require.NoError(t, err)

		assert.Equal(t, "/api/stacks/my-org/my-project/dev/hooks/deploy-hook", gotPath)
		assert.Equal(t, want, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusNotFound, `{"message":"not found"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.GetStackWebhook(t.Context(), stackID, "no-such-hook")
		assert.ErrorContains(t, err, "not found")
	})
}

func TestPingStackWebhook(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		want := apitype.WebhookDelivery{
			ID:           "delivery-123",
			Kind:         "ping",
			Payload:      `{"timestamp":1234567890,"message":"ping"}`,
			Timestamp:    1234567890,
			Duration:     42,
			RequestURL:   "https://example.com/webhook",
			ResponseCode: 200,
			ResponseBody: "ok",
		}

		var gotPath, gotMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(want)
			require.NoError(t, err)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.PingStackWebhook(t.Context(), stackID, "deploy-hook")
		require.NoError(t, err)

		assert.Equal(t, "POST", gotMethod)
		assert.Equal(t, "/api/stacks/my-org/my-project/dev/hooks/deploy-hook/ping", gotPath)
		assert.Equal(t, want, got)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusNotFound, `{"message":"not found"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.PingStackWebhook(t.Context(), stackID, "no-such-hook")
		assert.Error(t, err)
	})
}

func TestCreateStackWebhook(t *testing.T) {
	t.Parallel()

	stackID := StackIdentifier{
		Owner:   "my-org",
		Project: "my-project",
		Stack:   tokens.MustParseStackName("dev"),
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		format := "raw"
		want := apitype.Webhook{
			OrganizationName: "my-org",
			Name:             "new-hook",
			DisplayName:      "New Hook",
			PayloadURL:       "https://example.com/webhook",
			Active:           true,
			Format:           &format,
			HasSecret:        true,
		}

		var gotPath, gotMethod string
		var gotBody apitype.Webhook
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotMethod = r.Method
			err := json.NewDecoder(r.Body).Decode(&gotBody)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(want)
			require.NoError(t, err)
		}))
		defer srv.Close()

		req := apitype.Webhook{
			OrganizationName: "my-org",
			Name:             "new-hook",
			DisplayName:      "New Hook",
			PayloadURL:       "https://example.com/webhook",
			Active:           true,
			Format:           &format,
		}

		c := newMockClient(srv)
		got, err := c.CreateStackWebhook(t.Context(), stackID, req)
		require.NoError(t, err)

		assert.Equal(t, "POST", gotMethod)
		assert.Equal(t, "/api/stacks/my-org/my-project/dev/hooks", gotPath)
		assert.Equal(t, "new-hook", gotBody.Name)
		assert.Equal(t, "https://example.com/webhook", gotBody.PayloadURL)
		assert.Equal(t, want, got)
	})

	t.Run("conflict", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusConflict, `{"message":"webhook already exists"}`)
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.CreateStackWebhook(t.Context(), stackID, apitype.Webhook{
			Name:       "existing",
			PayloadURL: "https://example.com",
		})
		assert.ErrorContains(t, err, "webhook already exists")
	})
}
