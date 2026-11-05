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

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPolicyGroupStacks(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		want := cloudapitype.ListPolicyGroupStacksResponse{
			Stacks: []cloudapitype.PolicyGroupStack{
				{
					Name:           "prod",
					RoutingProject: "web",
					Source:         cloudapitype.PolicyGroupMemberSourceRule,
					Tags:           map[string]string{"env": "prod"},
				},
			},
			ContinuationToken: new("next"),
		}

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(want))
		}))
		defer server.Close()

		c := newMockClient(server)
		got, err := c.ListPolicyGroupStacks(t.Context(), "acme", "prod-policies", ListPolicyGroupMembersOptions{})
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, "/api/orgs/acme/policygroups/prod-policies/stacks", capturedURI)
	})

	t.Run("passes source, count and continuationToken as query string", func(t *testing.T) {
		t.Parallel()

		var capturedURI string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedURI = r.URL.RequestURI()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"stacks":[]}`))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListPolicyGroupStacks(t.Context(), "acme", "prod-policies", ListPolicyGroupMembersOptions{
			Source:            cloudapitype.PolicyGroupMemberSourceExplicit,
			Count:             50,
			ContinuationToken: new("abc"),
		})
		require.NoError(t, err)
		assert.Equal(t,
			"/api/orgs/acme/policygroups/prod-policies/stacks?continuationToken=abc&count=50&source=explicit",
			capturedURI,
		)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("policy group not found"))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListPolicyGroupStacks(t.Context(), "acme", "missing", ListPolicyGroupMembersOptions{})
		require.ErrorContains(t, err, "listing policy group stacks")
	})
}

func TestListPolicyGroupAccounts(t *testing.T) {
	t.Parallel()

	t.Run("returns parsed response", func(t *testing.T) {
		t.Parallel()

		want := cloudapitype.ListPolicyGroupAccountsResponse{
			Accounts: []cloudapitype.PolicyGroupAccount{
				{
					ID:     "acc-1",
					Name:   "aws-prod",
					Source: cloudapitype.PolicyGroupMemberSourceExplicit,
					Tags:   map[string]string{},
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
		got, err := c.ListPolicyGroupAccounts(t.Context(), "acme", "compliance", ListPolicyGroupMembersOptions{
			Source: cloudapitype.PolicyGroupMemberSourceRule,
			Count:  10,
		})
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Equal(t, "/api/orgs/acme/policygroups/compliance/accounts?count=10&source=rule", capturedURI)
	})

	t.Run("propagates HTTP errors", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid source"))
		}))
		defer server.Close()

		c := newMockClient(server)
		_, err := c.ListPolicyGroupAccounts(t.Context(), "acme", "compliance", ListPolicyGroupMembersOptions{})
		require.ErrorContains(t, err, "listing policy group accounts")
	})
}
