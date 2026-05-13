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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// envWebhookCapture records the method, path, and decoded body of the single
// request a test routes through the mock server. Tests assert against this
// shape so a regression in path encoding or request shape fails loudly.
type envWebhookCapture struct {
	method string
	path   string
	body   []byte
}

// newEnvHooksServer spins up an httptest.Server that records each request and
// responds with the supplied JSON-encoded payload. Use status to override the
// 200/204 default for sad-path tests.
func newEnvHooksServer(
	t *testing.T, captured *envWebhookCapture, status int, payload any,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*captured = envWebhookCapture{
			method: r.Method,
			path:   r.URL.EscapedPath(),
			body:   body,
		}
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			require.NoError(t, json.NewEncoder(w).Encode(payload))
		}
	}))
}

func TestEnvironmentWebhooks(t *testing.T) {
	t.Parallel()

	t.Run("ListEnvironmentWebhooks", func(t *testing.T) {
		t.Parallel()
		want := []apitype.EnvironmentWebhook{
			{Name: "a", DisplayName: "A", PayloadURL: "https://a", Active: true},
		}
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, want)
		defer srv.Close()

		got, err := newMockClient(srv).ListEnvironmentWebhooks(t.Context(), "acme", "proj", "env")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, http.MethodGet, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks", captured.path)
	})

	t.Run("GetEnvironmentWebhook", func(t *testing.T) {
		t.Parallel()
		want := apitype.EnvironmentWebhook{Name: "h", DisplayName: "H", PayloadURL: "https://h", Active: true}
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, want)
		defer srv.Close()

		got, err := newMockClient(srv).GetEnvironmentWebhook(t.Context(), "acme", "proj", "env", "h")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, http.MethodGet, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks/h", captured.path)
	})

	t.Run("CreateEnvironmentWebhook", func(t *testing.T) {
		t.Parallel()
		req := apitype.CreateEnvironmentWebhookRequest{
			Name:        "h",
			DisplayName: "H",
			PayloadURL:  "https://h",
			Active:      true,
			Format:      "raw",
			Filters:     []string{"a"},
		}
		resp := apitype.EnvironmentWebhook{Name: "h", DisplayName: "H", PayloadURL: "https://h", Active: true}

		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusCreated, resp)
		defer srv.Close()

		got, err := newMockClient(srv).CreateEnvironmentWebhook(t.Context(), "acme", "proj", "env", req)
		require.NoError(t, err)
		assert.Equal(t, resp, got)
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks", captured.path)

		var decoded apitype.CreateEnvironmentWebhookRequest
		require.NoError(t, json.Unmarshal(captured.body, &decoded))
		assert.Equal(t, req, decoded)
	})

	t.Run("UpdateEnvironmentWebhook only sends set fields", func(t *testing.T) {
		t.Parallel()

		active := false
		req := apitype.UpdateEnvironmentWebhookRequest{Active: &active}
		resp := apitype.EnvironmentWebhook{Name: "h", Active: false}

		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, resp)
		defer srv.Close()

		got, err := newMockClient(srv).UpdateEnvironmentWebhook(t.Context(), "acme", "proj", "env", "h", req)
		require.NoError(t, err)
		assert.Equal(t, resp, got)
		assert.Equal(t, http.MethodPatch, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks/h", captured.path)

		// Ensure that only `active` made it into the JSON body — none of the
		// other ternary fields should serialize.
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(captured.body, &decoded))
		assert.Equal(t, map[string]any{"active": false}, decoded)
	})

	t.Run("DeleteEnvironmentWebhook", func(t *testing.T) {
		t.Parallel()
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusNoContent, nil)
		defer srv.Close()

		require.NoError(t, newMockClient(srv).DeleteEnvironmentWebhook(t.Context(), "acme", "proj", "env", "h"))
		assert.Equal(t, http.MethodDelete, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks/h", captured.path)
	})

	t.Run("PingEnvironmentWebhook", func(t *testing.T) {
		t.Parallel()
		want := apitype.EnvironmentWebhookDelivery{
			ID: "d1", Kind: "ping", Timestamp: 1, Duration: 42,
			Payload: "{}", RequestURL: "https://h", RequestHeaders: "{}",
			ResponseCode: 200, ResponseHeaders: "{}", ResponseBody: "{}",
		}
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, want)
		defer srv.Close()

		got, err := newMockClient(srv).PingEnvironmentWebhook(t.Context(), "acme", "proj", "env", "h")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks/h/ping", captured.path)
	})

	t.Run("ListEnvironmentWebhookDeliveries", func(t *testing.T) {
		t.Parallel()
		want := []apitype.EnvironmentWebhookDelivery{
			{ID: "d1", Kind: "ping", ResponseCode: 200, Duration: 5},
			{ID: "d2", Kind: "stack.updated", ResponseCode: 500, Duration: 10},
		}
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, want)
		defer srv.Close()

		got, err := newMockClient(srv).ListEnvironmentWebhookDeliveries(t.Context(), "acme", "proj", "env", "h")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, http.MethodGet, captured.method)
		assert.Equal(t, "/api/esc/environments/acme/proj/env/hooks/h/deliveries", captured.path)
	})

	t.Run("path segments are URL-escaped", func(t *testing.T) {
		t.Parallel()
		var captured envWebhookCapture
		srv := newEnvHooksServer(t, &captured, http.StatusOK, []apitype.EnvironmentWebhook{})
		defer srv.Close()

		_, err := newMockClient(srv).ListEnvironmentWebhooks(t.Context(), "acme/team", "my proj", "env+1")
		require.NoError(t, err)
		assert.Equal(t,
			"/api/esc/environments/acme%2Fteam/my%20proj/env+1/hooks",
			captured.path,
		)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer srv.Close()

		_, err := newMockClient(srv).GetEnvironmentWebhook(t.Context(), "acme", "proj", "env", "missing")
		require.Error(t, err)
	})
}
