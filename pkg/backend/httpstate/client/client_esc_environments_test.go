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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestEnvironmentSchedules(t *testing.T) {
	t.Parallel()

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		want := apitype.ListScheduledActionsResponse{
			Schedules: []apitype.ScheduledAction{
				{ID: "s1", Kind: "environment_rotation", ScheduleCron: "0 9 * * *"},
				{ID: "s2", Kind: "environment_rotation", ScheduleOnce: "2030-01-01T00:00:00Z"},
			},
		}

		var capturedPath, capturedMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedMethod = r.Method
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer srv.Close()

		got, err := newMockClient(srv).ListEnvironmentSchedules(t.Context(), "acme", "p", "e")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, "/api/esc/environments/acme/p/e/schedules", capturedPath)
		assert.Equal(t, "GET", capturedMethod)
	})

	t.Run("list http error", func(t *testing.T) {
		t.Parallel()

		srv := newMockServer(http.StatusInternalServerError, `{"message":"internal error"}`)
		defer srv.Close()

		_, err := newMockClient(srv).ListEnvironmentSchedules(t.Context(), "acme", "p", "e")
		assert.Error(t, err)
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		want := apitype.ScheduledAction{ID: "s1", Kind: "environment_rotation", ScheduleCron: "0 9 * * *"}
		var capturedBody apitype.CreateEnvironmentScheduleRequest
		var capturedMethod, capturedPath string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedPath = r.URL.Path
			require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer srv.Close()

		req := apitype.CreateEnvironmentScheduleRequest{
			ScheduleCron:          "0 9 * * *",
			SecretRotationRequest: &apitype.CreateEnvironmentSecretRotationScheduleRequest{},
		}
		got, err := newMockClient(srv).CreateEnvironmentSchedule(t.Context(), "acme", "p", "e", req)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, "POST", capturedMethod)
		assert.Equal(t, "/api/esc/environments/acme/p/e/schedules", capturedPath)
		assert.Equal(t, "0 9 * * *", capturedBody.ScheduleCron)
		require.NotNil(t, capturedBody.SecretRotationRequest)
	})

	for _, op := range []struct {
		name, method, suffix string
		call                 func(context.Context, *Client) error
	}{
		{"pause", "POST", "/pause", func(ctx context.Context, c *Client) error {
			return c.PauseEnvironmentSchedule(ctx, "acme", "p", "e", "s1")
		}},
		{"resume", "POST", "/resume", func(ctx context.Context, c *Client) error {
			return c.ResumeEnvironmentSchedule(ctx, "acme", "p", "e", "s1")
		}},
		{"delete", "DELETE", "", func(ctx context.Context, c *Client) error {
			return c.DeleteEnvironmentSchedule(ctx, "acme", "p", "e", "s1")
		}},
	} {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			var capturedMethod, capturedPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				capturedPath = r.URL.Path
			}))
			defer srv.Close()

			require.NoError(t, op.call(t.Context(), newMockClient(srv)))
			assert.Equal(t, op.method, capturedMethod)
			assert.Equal(t, "/api/esc/environments/acme/p/e/schedules/s1"+op.suffix, capturedPath)
		})
	}

	t.Run("path escaping", func(t *testing.T) {
		t.Parallel()

		var capturedRawPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedRawPath = r.URL.EscapedPath()
		}))
		defer srv.Close()

		require.NoError(t, newMockClient(srv).DeleteEnvironmentSchedule(
			t.Context(), "acme", "my project", "env/with/slash", "id with space"))
		assert.Equal(t,
			"/api/esc/environments/acme/my%20project/env%2Fwith%2Fslash/schedules/id%20with%20space",
			capturedRawPath)
	})
}
