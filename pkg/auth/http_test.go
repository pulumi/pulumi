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
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func httpResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: http.NoBody}
}

func TestHTTPAuthRefreshPolicy(t *testing.T) {
	for _, tt := range []struct {
		name        string
		helper      credentialhelper.Response
		environment string
		lease       bool
		expiring    bool
		helperCalls int
		tokens      []string
		gates       []string
	}{
		{
			name: "helper token", helper: credentialhelper.Response{AccessToken: "helper"},
			helperCalls: 1, tokens: []string{"token helper", "token helper-refreshed"}, gates: []string{"", "new"},
		},
		{
			name: "expiry consumes the refresh", helper: credentialhelper.Response{AccessToken: "helper"},
			expiring: true, helperCalls: 1, tokens: []string{"token helper-refreshed"}, gates: []string{"new"},
		},
		{
			name: "headers keep the caller's token", helper: credentialhelper.Response{Headers: http.Header{"X-Gate": {"old"}}},
			helperCalls: 1, tokens: []string{"token stored", "token stored"}, gates: []string{"old", "new"},
		},
		{
			name: "environment token overrides helper token", helper: credentialhelper.Response{AccessToken: "helper"},
			environment: "environment", tokens: []string{"token stored"}, gates: []string{""},
		},
		{
			name: "environment token with headers", helper: credentialhelper.Response{
				AccessToken: "helper", Headers: http.Header{"X-Gate": {"old"}},
			},
			environment: "environment", helperCalls: 1, tokens: []string{"token stored", "token stored"},
			gates: []string{"old", "new"},
		},
		{
			name: "lease with headers", helper: credentialhelper.Response{
				AccessToken: "helper", Headers: http.Header{"X-Gate": {"old"}},
			},
			lease: true, helperCalls: 1, tokens: []string{"update-token lease", "update-token lease"},
			gates: []string{"old", "new"},
		},
		{
			name: "lease without headers", helper: credentialhelper.Response{AccessToken: "helper"},
			lease: true, tokens: []string{"update-token lease"}, gates: []string{""},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_ACCESS_TOKEN", tt.environment)
			if tt.expiring {
				tt.helper.ExpiresAt = new(time.Now().Add(10 * time.Second))
			}
			hadToken := tt.helper.AccessToken != ""
			var helperCalls int
			a := newHTTPAuth(testBackendURL, &tt.helper, func(
				_ context.Context, req credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				helperCalls++
				assert.Equal(t, testBackendURL, req.SelectedBackendURL)
				reason := credentialhelper.Rejected
				if tt.expiring {
					reason = credentialhelper.Expired
				}
				assert.Equal(t, reason, req.Reason)
				next := &credentialhelper.Response{Headers: http.Header{"X-Gate": {"new"}}}
				if hadToken {
					next.AccessToken = "helper-refreshed"
				}
				return next, nil
			}, &helperOutput{})
			var tokens, gates []string
			base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				tokens = append(tokens, req.Header.Get("Authorization"))
				gates = append(gates, req.Header.Get("X-Gate"))
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.NoError(t, req.Body.Close())
				assert.Equal(t, "request body", string(body))
				return httpResponse(http.StatusUnauthorized), nil
			})
			req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodPost,
				testBackendURL, strings.NewReader("request body"))
			require.NoError(t, err)
			req.Header.Set("Authorization", "token stored")
			if tt.lease {
				req.Header.Set("Authorization", "update-token lease")
			}
			resp, err := (&http.Client{Transport: a.Transport(base)}).Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Equal(t, tt.tokens, tokens)
			assert.Equal(t, tt.gates, gates)
			assert.Equal(t, tt.helperCalls, helperCalls)
		})
	}
}

func TestHTTPAuthSingleRefreshAcrossRequests(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	var calls int
	a := newHTTPAuth(testBackendURL, &credentialhelper.Response{Headers: http.Header{"X-Gate": {"old"}}},
		func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
			calls++
			return &credentialhelper.Response{Headers: http.Header{"X-Gate": {"new"}}}, nil
		}, &helperOutput{})
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusUnauthorized), nil
	})
	// An operation's requests share one refresh, although each gets its own transport.
	ctx := WithHelperAuth(t.Context())
	for range 2 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, testBackendURL, nil)
		require.NoError(t, err)
		resp, err := a.Transport(base).RoundTrip(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	assert.Equal(t, 1, calls)

	// A separate operation may refresh again.
	req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
	require.NoError(t, err)
	resp, err := a.Transport(base).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, 2, calls)
}

func TestHTTPAuthNilLeavesRequestsUnchanged(t *testing.T) {
	t.Parallel()
	var a *HTTPAuth
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) { return httpResponse(http.StatusOK), nil })
	assert.Empty(t, a.AccessToken())
	assert.IsType(t, base, a.Transport(base))
}

func TestHTTPAuthLeavesOtherRequestsAlone(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	a := newHTTPAuth(testBackendURL, &credentialhelper.Response{
		AccessToken: "helper", Headers: http.Header{"X-Gate": {"helper"}},
	}, func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
		t.Error("the helper must not run for a request that is not a backend API call")
		return nil, nil
	}, &helperOutput{})
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		assert.Empty(t, req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("X-Gate"))
		return httpResponse(http.StatusUnauthorized), nil
	})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testBackendURL+"/artifact", nil)
	require.NoError(t, err)
	resp, err := a.Transport(base).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHTTPAuthRefreshMustKeepToken(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	for _, next := range []*credentialhelper.Response{nil, {Headers: http.Header{"X-Gate": {"new"}}}} {
		a := newHTTPAuth(testBackendURL, &credentialhelper.Response{AccessToken: "helper"}, func(
			context.Context, credentialhelper.Request,
		) (*credentialhelper.Response, error) {
			return next, nil
		}, &helperOutput{})
		base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return httpResponse(http.StatusUnauthorized), nil
		})
		req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
		require.NoError(t, err)
		transport := a.Transport(base)
		_, err = transport.RoundTrip(req) //nolint:bodyclose // The refresh fails before a response is returned.
		require.ErrorContains(t, err, "refreshing credential helper credentials")
		// A retry of the same request reports the failure without running the helper again.
		_, retryErr := transport.RoundTrip(req) //nolint:bodyclose // As above.
		assert.Equal(t, err, retryErr)
		assert.Equal(t, "helper", a.AccessToken())
	}
}

func TestHTTPAuthRefreshMustNotStartSupplyingToken(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	a := newHTTPAuth(testBackendURL, &credentialhelper.Response{Headers: http.Header{"X-Gate": {"old"}}}, func(
		context.Context, credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		return &credentialhelper.Response{AccessToken: "late-token"}, nil
	}, &helperOutput{})
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusUnauthorized), nil
	})
	req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
	require.NoError(t, err)
	_, err = a.Transport(base).RoundTrip(req) //nolint:bodyclose // The refresh fails before a response is returned.
	require.ErrorContains(t, err, "must not start supplying an accessToken")
	assert.Empty(t, a.AccessToken())
}

func TestHTTPAuthConcurrentRefresh(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	for _, fail := range []bool{false, true} {
		const requests = 12
		var refreshCalls atomic.Int32
		refreshErr := errors.New("refresh unavailable")
		a := newHTTPAuth(testBackendURL, &credentialhelper.Response{
			AccessToken: "unchanged", Headers: http.Header{"X-Gate": {"old"}, "X-Removed": {"old"}},
		}, func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
			refreshCalls.Add(1)
			if fail {
				return nil, refreshErr
			}
			return &credentialhelper.Response{AccessToken: "unchanged", Headers: http.Header{"X-Gate": {"new"}}}, nil
		}, &helperOutput{})
		var initial sync.WaitGroup
		initial.Add(requests)
		base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "token unchanged", req.Header.Get("Authorization"))
			if req.Header.Get("X-Gate") == "old" {
				initial.Done()
				initial.Wait()
				return httpResponse(http.StatusUnauthorized), nil
			}
			assert.Empty(t, req.Header.Get("X-Removed"))
			return httpResponse(http.StatusOK), nil
		})
		results := make(chan error, requests)
		for range requests {
			req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
			require.NoError(t, err)
			go func() {
				resp, err := (&http.Client{Transport: a.Transport(base)}).Do(req)
				if err == nil {
					assert.Equal(t, http.StatusOK, resp.StatusCode)
					err = resp.Body.Close()
				}
				results <- err
			}()
		}
		for range requests {
			err := <-results
			if fail {
				require.ErrorIs(t, err, refreshErr)
			} else {
				require.NoError(t, err)
			}
		}
		assert.EqualValues(t, 1, refreshCalls.Load())
	}
}

func TestHTTPAuthReplacesExpiry(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	var calls int
	a := newHTTPAuth(testBackendURL,
		&credentialhelper.Response{AccessToken: "old", ExpiresAt: new(time.Now().Add(-time.Hour))},
		func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
			calls++
			return &credentialhelper.Response{AccessToken: "new"}, nil
		}, &helperOutput{})
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "token new", req.Header.Get("Authorization"))
		return httpResponse(http.StatusOK), nil
	})
	for range 2 {
		req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
		require.NoError(t, err)
		resp, err := (&http.Client{Transport: a.Transport(base)}).Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	assert.Equal(t, 1, calls)
	assert.Equal(t, "new", a.AccessToken())
}

//nolint:paralleltest // PULUMI_ACCESS_TOKEN is a process-wide environment variable.
func TestHTTPAuthRedirectScope(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	for _, target := range []string{
		"https://backend.example.com/next",
		"https://BACKEND.example.com:443/next",
		"https://backend.example.com:8443/next",
		"http://backend.example.com/next",
		"https://sub.backend.example.com/next",
		"https://other.example.com/next",
	} {
		t.Run(target, func(t *testing.T) {
			a := newHTTPAuth(testBackendURL,
				&credentialhelper.Response{AccessToken: "helper", Headers: http.Header{"X-Gate": {"secret"}}},
				nil, &helperOutput{})
			var calls int
			base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					assert.Equal(t, "token helper", req.Header.Get("Authorization"))
					assert.Equal(t, "secret", req.Header.Get("X-Gate"))
					resp := httpResponse(http.StatusTemporaryRedirect)
					resp.Header.Set("Location", target)
					return resp, nil
				}
				if target == "https://backend.example.com/next" || target == "https://BACKEND.example.com:443/next" {
					assert.Equal(t, "token helper", req.Header.Get("Authorization"))
					assert.Equal(t, "secret", req.Header.Get("X-Gate"))
				} else {
					assert.Empty(t, req.Header.Get("Authorization"))
					assert.Empty(t, req.Header.Get("X-Gate"))
				}
				return httpResponse(http.StatusOK), nil
			})
			req, err := http.NewRequestWithContext(WithHelperAuth(t.Context()), http.MethodGet, testBackendURL, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "token stored")
			resp, err := (&http.Client{Transport: a.Transport(base)}).Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, 2, calls)
			assert.Equal(t, "token stored", req.Header.Get("Authorization"))
			assert.Empty(t, req.Header.Get("X-Gate"))
		})
	}
}
