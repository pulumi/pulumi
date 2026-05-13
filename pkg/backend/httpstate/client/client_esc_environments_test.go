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

func TestListEnvironmentReferrers(t *testing.T) {
	t.Parallel()

	t.Run("parses response and forwards query params", func(t *testing.T) {
		t.Parallel()

		want := apitype.ListEnvironmentReferrersResponse{
			Referrers: map[string][]apitype.EnvironmentReferrer{
				"latest": {
					{Stack: &apitype.EnvironmentStackReferrer{
						Project: "p", Stack: "dev", Version: 4,
					}},
				},
			},
			ContinuationToken: "next-page",
		}

		var capturedURL string
		var capturedMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURL = r.URL.String()
			capturedMethod = r.Method
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer srv.Close()

		c := newMockClient(srv)
		got, err := c.ListEnvironmentReferrers(t.Context(), "acme", "my-project", "my-env",
			ListEnvironmentReferrersOptions{
				Count:                  50,
				AllRevisions:           true,
				LatestStackVersionOnly: true,
				ContinuationToken:      "abc",
			})
		require.NoError(t, err)
		assert.Equal(t, want, got)

		assert.Equal(t, http.MethodGet, capturedMethod)
		assert.Contains(t, capturedURL, "/api/esc/environments/acme/my-project/my-env/referrers")
		assert.Contains(t, capturedURL, "count=50")
		assert.Contains(t, capturedURL, "allRevisions=true")
		assert.Contains(t, capturedURL, "latestStackVersionOnly=true")
		assert.Contains(t, capturedURL, "continuationToken=abc")
	})

	t.Run("zero-valued options send no query string", func(t *testing.T) {
		t.Parallel()

		var capturedQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(apitype.ListEnvironmentReferrersResponse{}))
		}))
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.ListEnvironmentReferrers(t.Context(), "acme", "p", "e",
			ListEnvironmentReferrersOptions{})
		require.NoError(t, err)
		assert.Empty(t, capturedQuery)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		c := newMockClient(srv)
		_, err := c.ListEnvironmentReferrers(t.Context(), "acme", "p", "missing",
			ListEnvironmentReferrersOptions{})
		require.Error(t, err)
	})
}
