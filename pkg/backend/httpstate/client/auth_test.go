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
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	escclient "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestHTTPAuthSharedWithESC(t *testing.T) {
	t.Parallel()
	var exchanges, requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			exchanges.Add(1)
			assert.Empty(t, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken: "fresh", RefreshToken: "rotated",
			}))
			return
		}
		requests.Add(1)
		if r.Header.Get("Authorization") != "token fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(serviceUser{GitHubLogin: "alice"}))
	}))
	t.Cleanup(server.Close)
	var savedToken, savedRefreshToken string
	pc := NewClient(server.URL, "stale", false, nil).WithRefresh("refresh", func(at string, _ time.Time, rt string) error {
		savedToken, savedRefreshToken = at, rt
		return nil
	})
	esc := escclient.NewWithHTTPAuth(UserAgent(), server.URL, false, pc.HTTPAuth())
	name, _, _, err := esc.GetPulumiAccountDetails(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "alice", name)
	name, _, _, err = pc.GetPulumiAccountDetails(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "alice", name)
	assert.EqualValues(t, 1, exchanges.Load())
	assert.EqualValues(t, 3, requests.Load())
	assert.Equal(t, "fresh", savedToken)
	assert.Equal(t, "rotated", savedRefreshToken)
}

func TestHTTPAuthOAuthFailurePreservesRejection(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"exchange", "writeback", "rejected again"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			var exchanges, requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/oauth/token" {
					exchanges.Add(1)
					if failure == "exchange" {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					require.NoError(t, json.NewEncoder(w).Encode(apitype.TokenExchangeGrantResponse{AccessToken: "new"}))
					return
				}
				requests.Add(1)
				w.WriteHeader(http.StatusUnauthorized)
				require.NoError(t, json.NewEncoder(w).Encode(apitype.ErrorResponse{
					Code: 401, Message: "SAML SSO authentication is required",
					Errors: []apitype.RequestError{{
						Resource: "organization", Attribute: new("myorg"), ErrorType: "saml_reauth_required",
					}},
				}))
			}))
			t.Cleanup(server.Close)
			target, err := url.Parse(server.URL)
			require.NoError(t, err)
			httpClient := &http.Client{Transport: &errorTransport{roundTripFunc: func(r *http.Request) (*http.Response, error) {
				r = r.Clone(r.Context())
				r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
				return http.DefaultTransport.RoundTrip(r)
			}}}
			pc := NewClient("https://api.pulumi.com", "old", false, nil).WithHTTPClient(httpClient)
			pc.WithRefresh("refresh", func(string, time.Time, string) error {
				if failure == "writeback" {
					return errors.New("disk full")
				}
				return nil
			})
			_, _, _, err = pc.GetPulumiAccountDetails(t.Context())
			var loginErr backenderr.LoginRequiredError
			require.ErrorAs(t, err, &loginErr)
			assert.Contains(t, loginErr.ReauthURL, "/signin/sso/myorg/reauth")
			assert.EqualValues(t, 1, exchanges.Load())
			wantRequests := 1
			if failure == "rejected again" {
				wantRequests = 2
			}
			assert.EqualValues(t, wantRequests, requests.Load())
		})
	}
}

func TestWithHTTPAuthReplacesAccount(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"", "replacement"} {
		t.Run(token, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.NotEqual(t, "/api/oauth/token", r.URL.Path)
				want := ""
				if token != "" {
					want = "token " + token
				}
				assert.Equal(t, want, r.Header.Get("Authorization"))
				w.WriteHeader(http.StatusUnauthorized)
			}))
			t.Cleanup(server.Close)
			pc := NewClient(server.URL, "stored", false, nil).WithRefresh("refresh", func(string, time.Time, string) error {
				t.Error("replaced account credentials must not be refreshed")
				return nil
			}).WithHTTPAuth(auth.NewHTTPAuth(server.URL, token, nil))
			_, _, _, err := pc.GetPulumiAccountDetails(t.Context())
			require.Error(t, err)
		})
	}
}
