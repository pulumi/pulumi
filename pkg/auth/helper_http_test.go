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

package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
)

func TestHelperHTTPAuthRefresh(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		token  string
		reason refreshReason
	}{
		{name: "token and headers", token: "helper-token", reason: credentialsExpired},
		{name: "headers only", reason: credentialsRejected},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			expiry := time.Now().Add(time.Hour)
			initial := &credentialhelper.Response{
				AccessToken: test.token, Headers: http.Header{"X-Gate": {"old"}}, ExpiresAt: &expiry,
			}
			next := &credentialhelper.Response{Headers: http.Header{"X-Gate": {"new"}}, ExpiresAt: &expiry}
			if test.token != "" {
				next.AccessToken = "refreshed-token"
			}
			a := newHelperHTTPAuth(testBackendURL, initial, func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				assert.Equal(t, credentialhelper.Request{
					SelectedBackendURL: testBackendURL, Reason: credentialhelper.Reason(test.reason),
				}, request)
				return next, nil
			})
			assert.Equal(t, httpCredentials{
				accessToken: test.token, headers: initial.Headers, expiresAt: &expiry,
			}, a.current)
			refreshed, err := a.refresh(t.Context(), test.reason)
			require.NoError(t, err)
			assert.Equal(t, httpCredentials{
				accessToken: next.AccessToken, headers: next.Headers, expiresAt: &expiry,
			}, refreshed)
		})
	}
}

func TestHelperHTTPAuthRefreshRequiresConsistentToken(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		token   string
		next    *credentialhelper.Response
		message string
	}{
		{name: "declines", token: "helper", message: "helper returned no HTTP credentials"},
		{
			name: "stops supplying token", token: "helper",
			next:    &credentialhelper.Response{Headers: http.Header{"X-Gate": {"new"}}},
			message: "helper must continue supplying an accessToken",
		},
		{
			name: "starts supplying token", next: &credentialhelper.Response{AccessToken: "late-token"},
			message: "helper must not start supplying an accessToken on refresh",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			a := newHelperHTTPAuth(testBackendURL, &credentialhelper.Response{
				AccessToken: test.token, Headers: http.Header{"X-Gate": {"old"}},
			}, func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
				return test.next, nil
			})
			err := a.renew(t.Context(), 0, credentialsRejected)
			require.ErrorContains(t, err, "refreshing credential helper credentials: "+test.message)
			assert.Equal(t, test.token, a.AccessToken())
		})
	}
}
