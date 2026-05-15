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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestGetOrgUsageSummary(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		want := apitype.OrgUsageSummaryResponse{
			Summary: []apitype.OrgResourceCountSummary{
				{
					Year:          2026,
					Month:         ptr(5),
					Day:           ptr(1),
					Resources:     1200,
					ResourceHours: 28800,
				},
				{
					Year:          2026,
					Month:         ptr(5),
					Day:           ptr(2),
					Resources:     1300,
					ResourceHours: 31200,
				},
			},
		}

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		c := newMockClient(server)
		got, err := c.GetOrgUsageSummary(t.Context(), "acme", apitype.OrgUsageSummaryParams{})
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, "/api/orgs/acme/resources/summary", capturedURI)
	})

	t.Run("passes granularity and lookback as query string", func(t *testing.T) {
		t.Parallel()

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"summary":[]}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.GetOrgUsageSummary(t.Context(), "acme", apitype.OrgUsageSummaryParams{
			Granularity:   "daily",
			LookbackDays:  7,
			LookbackStart: 1_700_000_000,
		})
		require.NoError(t, err)
		assert.Equal(t,
			"/api/orgs/acme/resources/summary?granularity=daily&lookbackDays=7&lookbackStart=1700000000",
			capturedURI,
		)
	})

	t.Run("escapes org name in path", func(t *testing.T) {
		t.Parallel()

		var capturedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"summary":[]}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.GetOrgUsageSummary(t.Context(), "acme/inc", apitype.OrgUsageSummaryParams{})
		require.NoError(t, err)
		assert.Equal(t, "/api/orgs/acme%2Finc/resources/summary", capturedPath)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid query parameter"))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.GetOrgUsageSummary(t.Context(), "acme", apitype.OrgUsageSummaryParams{
			LookbackDays: -1,
		})
		require.Error(t, err)
	})
}
