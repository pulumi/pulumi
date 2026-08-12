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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOrganizationMembers(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		want := apitype.ListOrganizationMembersResponse{
			Members: []apitype.OrganizationMember{
				{
					Role: "admin",
					User: apitype.UserInfo{
						Name:        "Alice",
						GitHubLogin: "alice",
						AvatarURL:   "https://example.com/a.png",
					},
					Created:       "2026-05-01T12:00:00Z",
					KnownToPulumi: true,
					VirtualAdmin:  false,
					Links:         &apitype.MemberLinks{Self: "/api/orgs/acme/members/alice"},
					FGARole: apitype.FGARole{
						ID:         "role-admin",
						Name:       "admin",
						ModifiedAt: "2026-04-01T00:00:00Z",
					},
				},
				{
					Role: "member",
					User: apitype.UserInfo{
						Name:        "Bob",
						GitHubLogin: "bob",
					},
					Created:       "2026-04-15T08:00:00Z",
					KnownToPulumi: true,
					FGARole: apitype.FGARole{
						ID:         "role-member",
						Name:       "member",
						ModifiedAt: "2026-04-01T00:00:00Z",
					},
				},
			},
			ContinuationToken: "next-page-token",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		c := newMockClient(server)
		got, err := c.ListOrganizationMembers(t.Context(), "acme", "", nil)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("passes continuationToken and mode as query string", func(t *testing.T) {
		t.Parallel()

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"members":[]}`))
		}))
		defer server.Close()

		token := "abc"
		c := newMockClient(server)
		_, err := c.ListOrganizationMembers(t.Context(), "acme", "backend", &token)
		require.NoError(t, err)
		assert.Equal(t,
			"/api/orgs/acme/members?continuationToken=abc&type=backend",
			capturedURI,
		)
	})

	t.Run("omits zero-valued query params", func(t *testing.T) {
		t.Parallel()

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"members":[]}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListOrganizationMembers(t.Context(), "acme", "", nil)
		require.NoError(t, err)
		assert.Equal(t, "/api/orgs/acme/members", capturedURI)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid query parameter"))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListOrganizationMembers(t.Context(), "acme", "nope", nil)
		require.Error(t, err)
	})
}
