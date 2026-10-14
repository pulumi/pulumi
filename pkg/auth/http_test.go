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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func httpResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: http.NoBody}
}

const testHTTPBackendURL = "https://backend.example.com"

func TestHTTPAuthReplay(t *testing.T) {
	t.Parallel()
	for _, replayable := range []bool{true, false} {
		t.Run(strconv.FormatBool(replayable), func(t *testing.T) {
			t.Parallel()
			var refreshes, requests int
			a := NewHTTPAuth(testHTTPBackendURL, "old", func(context.Context) (string, error) {
				refreshes++
				return "new", nil
			})
			base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.NoError(t, req.Body.Close())
				assert.Equal(t, "body", string(body))
				if requests == 1 {
					assert.Equal(t, "token old", req.Header.Get("Authorization"))
				} else {
					assert.Equal(t, "token new", req.Header.Get("Authorization"))
				}
				return httpResponse(http.StatusUnauthorized), nil
			})
			req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodPost,
				testHTTPBackendURL, strings.NewReader("body"))
			require.NoError(t, err)
			if !replayable {
				req.GetBody = nil
			}
			resp, err := (&http.Client{Transport: Transport(base)}).Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			if replayable {
				assert.Equal(t, 1, refreshes)
				assert.Equal(t, 2, requests)
			} else {
				assert.Zero(t, refreshes)
				assert.Equal(t, 1, requests)
			}
			assert.Empty(t, req.Header.Get("Authorization"))
		})
	}
}

func TestHTTPAuthUpdateLease(t *testing.T) {
	t.Parallel()
	for _, withHeaders := range []bool{false, true} {
		t.Run(strconv.FormatBool(withHeaders), func(t *testing.T) {
			t.Parallel()
			initial := httpCredentials{accessToken: "account"}
			if withHeaders {
				initial.headers = http.Header{"X-Gate": {"old"}}
			}
			var refreshes int
			a := authWithCredentials(initial, func(context.Context, refreshReason) (httpCredentials, error) {
				refreshes++
				return httpCredentials{accessToken: "new", headers: http.Header{"X-Gate": {"new"}}}, nil
			})
			req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "update-token lease")
			var headers []string
			resp, err := Transport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "update-token lease", req.Header.Get("Authorization"))
				headers = append(headers, req.Header.Get("X-Gate"))
				return httpResponse(http.StatusUnauthorized), nil
			})).RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			if withHeaders {
				assert.Equal(t, 1, refreshes)
				assert.Equal(t, []string{"old", "new"}, headers)
			} else {
				assert.Zero(t, refreshes)
				assert.Equal(t, []string{""}, headers)
			}
		})
	}
}

func authWithCredentials(
	initial httpCredentials, refresh func(context.Context, refreshReason) (httpCredentials, error),
) *HTTPAuth {
	a := NewHTTPAuth(testHTTPBackendURL, initial.accessToken, nil)
	a.current = initial
	a.refresh = refresh
	return a
}

func TestHTTPAuthHeadersOnly(t *testing.T) {
	t.Parallel()
	a := authWithCredentials(httpCredentials{headers: http.Header{"X-Gate": {"secret"}}}, nil)
	req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "token stale")
	resp, err := Transport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		assert.Empty(t, req.Header.Get("Authorization"))
		assert.Equal(t, []string{"secret"}, req.Header.Values("X-Gate"))
		return httpResponse(http.StatusOK), nil
	})).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestHTTPAuthSingleRefreshAcrossRequests(t *testing.T) {
	t.Parallel()
	for _, expiring := range []bool{false, true} {
		t.Run(strconv.FormatBool(expiring), func(t *testing.T) {
			t.Parallel()
			var calls int
			initial := httpCredentials{headers: http.Header{"X-Gate": {"old"}}}
			if expiring {
				initial.expiresAt = new(time.Now().Add(10 * time.Second))
			}
			a := authWithCredentials(initial,
				func(_ context.Context, reason refreshReason) (httpCredentials, error) {
					if expiring && calls == 0 {
						assert.Equal(t, credentialsExpired, reason)
					} else {
						assert.Equal(t, credentialsRejected, reason)
					}
					calls++
					return httpCredentials{headers: http.Header{"X-Gate": {"new"}}}, nil
				})
			base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return httpResponse(http.StatusUnauthorized), nil
			})
			// An operation's requests share one refresh, although each gets its own transport.
			ctx := WithHTTPAuth(t.Context(), a)
			for range 2 {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, testHTTPBackendURL, nil)
				require.NoError(t, err)
				resp, err := Transport(base).RoundTrip(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			}
			assert.Equal(t, 1, calls)

			// A separate operation may refresh again.
			req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
			require.NoError(t, err)
			resp, err := Transport(base).RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, 2, calls)
		})
	}
}

func TestHTTPAuthLeavesOtherRequestsAlone(t *testing.T) {
	t.Parallel()
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		assert.Empty(t, req.Header.Get("Authorization"))
		assert.Empty(t, req.Header.Get("X-Gate"))
		return httpResponse(http.StatusUnauthorized), nil
	})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testHTTPBackendURL+"/artifact", nil)
	require.NoError(t, err)
	resp, err := Transport(base).RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHTTPAuthConcurrentRefresh(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		const requests = 12
		var refreshCalls atomic.Int32
		refreshErr := errors.New("refresh unavailable")
		a := authWithCredentials(httpCredentials{
			accessToken: "unchanged", headers: http.Header{"X-Gate": {"old"}, "X-Removed": {"old"}},
		}, func(context.Context, refreshReason) (httpCredentials, error) {
			refreshCalls.Add(1)
			if fail {
				return httpCredentials{}, refreshErr
			}
			return httpCredentials{accessToken: "unchanged", headers: http.Header{"X-Gate": {"new"}}}, nil
		})
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
			req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
			require.NoError(t, err)
			go func() {
				resp, err := (&http.Client{Transport: Transport(base)}).Do(req)
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

func TestHTTPAuthConcurrentExpiry(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			const requests = 12
			var calls atomic.Int32
			release := make(chan struct{})
			refreshErr := errors.New("refresh unavailable")
			a := authWithCredentials(httpCredentials{
				accessToken: "expired", expiresAt: new(time.Now().Add(-time.Hour)),
			}, func(context.Context, refreshReason) (httpCredentials, error) {
				calls.Add(1)
				<-release
				if fail {
					return httpCredentials{}, refreshErr
				}
				return httpCredentials{accessToken: "renewed"}, nil
			})
			base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "token renewed", req.Header.Get("Authorization"))
				return httpResponse(http.StatusOK), nil
			})
			request := func(ctx context.Context) error {
				req, err := http.NewRequestWithContext(WithHTTPAuth(ctx, a), http.MethodGet, testHTTPBackendURL, nil)
				require.NoError(t, err)
				resp, err := (&http.Client{Transport: Transport(base)}).Do(req)
				if err == nil {
					err = resp.Body.Close()
				}
				return err
			}
			results := make(chan error, requests)
			for range requests {
				go func() { results <- request(t.Context()) }()
			}
			synctest.Wait()
			assert.EqualValues(t, 1, calls.Load())

			// A waiter may cancel without cancelling the shared refresh.
			ctx, cancel := context.WithCancel(t.Context())
			cancelled := make(chan error, 1)
			go func() { cancelled <- request(ctx) }()
			synctest.Wait()
			cancel()
			synctest.Wait()
			require.ErrorIs(t, <-cancelled, context.Canceled)

			close(release)
			for range requests {
				err := <-results
				if fail {
					require.ErrorIs(t, err, refreshErr)
				} else {
					require.NoError(t, err)
				}
			}
			assert.EqualValues(t, 1, calls.Load())
			if fail {
				// A later operation can retry after the shared failure.
				require.ErrorIs(t, request(t.Context()), refreshErr)
				assert.EqualValues(t, 2, calls.Load())
			}
		})
	}
}

func TestHTTPAuthReplacesExpiry(t *testing.T) {
	t.Parallel()
	var calls int
	a := authWithCredentials(
		httpCredentials{accessToken: "old", expiresAt: new(time.Now().Add(-time.Hour))},
		func(context.Context, refreshReason) (httpCredentials, error) {
			calls++
			return httpCredentials{accessToken: "new"}, nil
		})
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "token new", req.Header.Get("Authorization"))
		return httpResponse(http.StatusOK), nil
	})
	for range 2 {
		req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
		require.NoError(t, err)
		resp, err := (&http.Client{Transport: Transport(base)}).Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	assert.Equal(t, 1, calls)
	assert.Equal(t, "new", a.AccessToken())
}

func TestHTTPAuthRedirectScope(t *testing.T) {
	t.Parallel()
	for _, target := range []string{
		"https://backend.example.com/next",
		"https://BACKEND.example.com:443/next",
		"https://backend.example.com:8443/next",
		"http://backend.example.com/next",
		"https://sub.backend.example.com/next",
		"https://other.example.com/next",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			a := authWithCredentials(
				httpCredentials{accessToken: "secret", headers: http.Header{"X-Gate": {"secret"}}},
				nil)
			var calls int
			base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					assert.Equal(t, "token secret", req.Header.Get("Authorization"))
					assert.Equal(t, "secret", req.Header.Get("X-Gate"))
					resp := httpResponse(http.StatusTemporaryRedirect)
					resp.Header.Set("Location", target)
					return resp, nil
				}
				if target == "https://backend.example.com/next" || target == "https://BACKEND.example.com:443/next" {
					assert.Equal(t, "token secret", req.Header.Get("Authorization"))
					assert.Equal(t, "secret", req.Header.Get("X-Gate"))
				} else {
					assert.Empty(t, req.Header.Get("Authorization"))
					assert.Empty(t, req.Header.Get("X-Gate"))
				}
				return httpResponse(http.StatusOK), nil
			})
			req, err := http.NewRequestWithContext(WithHTTPAuth(t.Context(), a), http.MethodGet, testHTTPBackendURL, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "token stored")
			resp, err := (&http.Client{Transport: Transport(base)}).Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			assert.Equal(t, 2, calls)
			assert.Equal(t, "token stored", req.Header.Get("Authorization"))
			assert.Empty(t, req.Header.Get("X-Gate"))
		})
	}
}
