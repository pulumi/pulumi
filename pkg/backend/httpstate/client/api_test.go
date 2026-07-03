// Copyright 2023, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestRetryPolicy_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give retryPolicy
		want string
	}{
		{give: retryNone, want: "none"},
		{give: retryGetMethod, want: "get"},
		{give: retryAllMethods, want: "all"},
		{give: retryPolicy(42), want: "retryPolicy(42)"}, // unknown
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			got := tt.give.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetryPolicy_shouldRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		policy retryPolicy
		method string // HTTP method
		want   bool
	}{
		{
			desc:   "none/GET",
			policy: retryNone,
			method: http.MethodGet,
			want:   false,
		},
		{
			desc:   "none/POST",
			policy: retryNone,
			method: http.MethodPost,
			want:   false,
		},
		{
			desc:   "get/GET",
			policy: retryGetMethod,
			method: http.MethodGet,
			want:   true,
		},
		{
			desc:   "get/POST",
			policy: retryGetMethod,
			method: http.MethodPost,
			want:   false,
		},
		{
			desc:   "all/GET",
			policy: retryAllMethods,
			method: http.MethodGet,
			want:   true,
		},
		{
			desc:   "all/POST",
			policy: retryAllMethods,
			method: http.MethodPost,
			want:   true,
		},

		// Sanity check: default is get
		{
			desc: "default/GET",
			// Don't set policy field;
			// zero value should be retryGetMethod.
			method: http.MethodGet,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := tt.policy.shouldRetry(&http.Request{
				Method: tt.method,
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHTTPClientUserAgent(t *testing.T) {
	t.Parallel()

	var inReq *http.Request
	client := &defaultHTTPClient{
		&http.Client{
			Transport: &errorTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					inReq = req

					// Return a response with a failing body reader
					return &http.Response{
						StatusCode: 200,
						Status:     "200 OK",
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			},
		},
	}

	req, err := http.NewRequest("GET", "/some/url", nil)
	require.NoError(t, err)

	_, err = client.Do(req, retryAllMethods)
	require.NoError(t, err)

	assert.Equal(t, UserAgent(), inReq.Header.Get("User-Agent"))
}

//nolint:paralleltest // mutates the package-level state observed by UserAgent()
func TestUserAgentExtras(t *testing.T) {
	t.Cleanup(func() {
		SetUserAgentCommand("")
		SetUserAgentAIAgent("")
	})

	SetUserAgentCommand("")
	SetUserAgentAIAgent("")
	base := UserAgent()
	assert.Regexp(t, `^pulumi-cli/1 \([^()]*\)$`, base)
	assert.NotContains(t, base, "cmd=")
	assert.NotContains(t, base, "agent=")

	SetUserAgentCommand("stack ls")
	withCmd := UserAgent()
	assert.Contains(t, withCmd, "; cmd=stack-ls)")
	assert.NotContains(t, withCmd, "agent=")

	SetUserAgentAIAgent("claude")
	withBoth := UserAgent()
	assert.Contains(t, withBoth, "; cmd=stack-ls;")
	assert.Contains(t, withBoth, "; agent=claude)")

	SetUserAgentCommand("")
	withAgentOnly := UserAgent()
	assert.NotContains(t, withAgentOnly, "cmd=")
	assert.Contains(t, withAgentOnly, "; agent=claude)")

	SetUserAgentCommand("weird (cmd; with) bits")
	assert.Contains(t, UserAgent(), "; cmd=weird-cmd-with-bits;")
}

func TestPulumiAPICall_401_LoginRequired(t *testing.T) {
	t.Parallel()

	t.Run("401_Generic", func(t *testing.T) {
		t.Parallel()

		respBody, _ := json.Marshal(apitype.ErrorResponse{
			Code:    401,
			Message: "Unauthorized",
		})
		httpClient := &defaultHTTPClient{
			&http.Client{
				Transport: &errorTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 401,
							Status:     "401 Unauthorized",
							Header:     http.Header{},
							Body:       io.NopCloser(bytes.NewReader(respBody)),
						}, nil
					},
				},
			},
		}

		_, _, err := pulumiAPICall(
			t.Context(), opentracing.NoopTracer{}.StartSpan("test"),
			diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			httpClient, "https://api.pulumi.com", "GET", "/api/test", nil,
			apiAccessToken("some-token"), httpCallOptions{},
		)

		var loginErr backenderr.LoginRequiredError
		require.ErrorAs(t, err, &loginErr)
		assert.Empty(t, loginErr.ReauthURL, "A 401 with no SAML error should return a generic LoginRequiredError")
	})

	for _, errorType := range []apitype.ErrorType{"saml_reauth_required", "saml_login_required"} {
		t.Run("401_SAML_"+string(errorType), func(t *testing.T) {
			t.Parallel()

			// A 401 with a SAML error type should include the reauth URL.
			orgName := "myorg"
			respBody, _ := json.Marshal(apitype.ErrorResponse{
				Code:    401,
				Message: "SAML SSO authentication is required.",
				Errors: []apitype.RequestError{
					{
						Resource:  "organization",
						Attribute: &orgName,
						ErrorType: errorType,
					},
				},
			})
			httpClient := &defaultHTTPClient{
				&http.Client{
					Transport: &errorTransport{
						roundTripFunc: func(req *http.Request) (*http.Response, error) {
							return &http.Response{
								StatusCode: 401,
								Status:     "401 Unauthorized",
								Header:     http.Header{},
								Body:       io.NopCloser(bytes.NewReader(respBody)),
							}, nil
						},
					},
				},
			}

			_, _, err := pulumiAPICall(
				t.Context(), opentracing.NoopTracer{}.StartSpan("test"),
				diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
				httpClient, "https://api.pulumi.com", "GET", "/api/test", nil,
				apiAccessToken("some-token"), httpCallOptions{},
			)

			var loginErr backenderr.LoginRequiredError
			require.ErrorAs(t, err, &loginErr)
			assert.Equal(t, "https://app.pulumi.com/signin/sso/myorg/reauth", loginErr.ReauthURL,
				"the error includes the url")
		})
	}

	t.Run("401_EmptyCredentials", func(t *testing.T) {
		t.Parallel()

		httpClient := &defaultHTTPClient{
			&http.Client{
				Transport: &errorTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 401,
							Status:     "401 Unauthorized",
							Header:     http.Header{},
							Body:       io.NopCloser(bytes.NewReader(nil)),
						}, nil
					},
				},
			},
		}

		_, _, err := pulumiAPICall(
			t.Context(), opentracing.NoopTracer{}.StartSpan("test"),
			diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			httpClient, "https://api.pulumi.com", "GET", "/api/test", nil,
			apiAccessToken(""), httpCallOptions{},
		)

		assert.True(t, errors.Is(err, backenderr.LoginRequiredError{}))
	})
}

func TestCall_RefreshOn401(t *testing.T) {
	t.Parallel()

	// errBody401 is the on-wire shape the service returns for an expired access token.
	errBody401, err := json.Marshal(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
	require.NoError(t, err)

	// newRESTClient builds a defaultRESTClient whose underlying HTTP transport is the given func.
	newRESTClient := func(rt func(req *http.Request) (*http.Response, error)) *defaultRESTClient {
		return &defaultRESTClient{
			client: &defaultHTTPClient{
				&http.Client{Transport: &errorTransport{roundTripFunc: rt}},
			},
		}
	}

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	t.Run("refreshes once and retries the request when the access token is rejected", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		var auths []string
		authsMu := make(chan struct{}, 1)
		authsMu <- struct{}{}

		rest := newRESTClient(func(req *http.Request) (*http.Response, error) {
			<-authsMu
			auths = append(auths, req.Header.Get("Authorization"))
			authsMu <- struct{}{}
			n := calls.Add(1)
			if n == 1 {
				return &http.Response{
					StatusCode: 401, Status: "401 Unauthorized",
					Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errBody401)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200, Status: "200 OK",
				Header: http.Header{}, Body: io.NopCloser(bytes.NewReader([]byte(`{}`))),
			}, nil
		})

		var refreshCalls atomic.Int32
		tok := &refreshableAPIAccessToken{
			accessToken:  "stale",
			refreshToken: "rt",
			refresh: func(_ context.Context, rt string) (string, time.Time, string, error) {
				refreshCalls.Add(1)
				assert.Equal(t, "rt", rt, "the wrapper sends the current refresh token")
				return "fresh", time.Time{}, "rt", nil
			},
			writeback: func(at string, _ time.Time, rt string) error { return nil },
		}

		err := rest.Call(t.Context(), sink,
			"https://api.example.com", "GET", "/api/test", nil, nil, nil, tok, httpCallOptions{})
		require.NoError(t, err)
		assert.Equal(t, int32(2), calls.Load(), "the request should be tried twice")
		assert.Equal(t, int32(1), refreshCalls.Load(), "refresh should fire exactly once")
		require.Len(t, auths, 2)
		assert.Equal(t, "token stale", auths[0], "first attempt uses the stale token")
		assert.Equal(t, "token fresh", auths[1], "retry uses the refreshed token")
	})

	t.Run("surfaces LoginRequiredError when the refresh itself fails", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		rest := newRESTClient(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{
				StatusCode: 401, Status: "401 Unauthorized",
				Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errBody401)),
			}, nil
		})

		tok := &refreshableAPIAccessToken{
			accessToken:  "stale",
			refreshToken: "rt-revoked",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				return "", time.Time{}, "", errors.New("invalid_grant")
			},
			writeback: func(at string, _ time.Time, rt string) error { return nil },
		}

		err := rest.Call(t.Context(), sink,
			"https://api.example.com", "GET", "/api/test", nil, nil, nil, tok, httpCallOptions{})
		require.Error(t, err)
		var loginErr backenderr.LoginRequiredError
		assert.ErrorAs(t, err, &loginErr)
		assert.Equal(t, int32(1), calls.Load(), "no retry when the refresh attempt fails")
	})

	t.Run("plain (non-refreshable) tokens fall through to LoginRequiredError unchanged", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		rest := newRESTClient(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{
				StatusCode: 401, Status: "401 Unauthorized",
				Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errBody401)),
			}, nil
		})

		err := rest.Call(t.Context(), sink,
			"https://api.example.com", "GET", "/api/test", nil, nil, nil,
			apiAccessToken("plain"), httpCallOptions{})
		require.Error(t, err)
		var loginErr backenderr.LoginRequiredError
		assert.ErrorAs(t, err, &loginErr)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("retries at most once even if the refreshed token is also rejected", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		rest := newRESTClient(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{
				StatusCode: 401, Status: "401 Unauthorized",
				Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errBody401)),
			}, nil
		})

		var refreshCalls atomic.Int32
		tok := &refreshableAPIAccessToken{
			accessToken:  "stale",
			refreshToken: "rt",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				refreshCalls.Add(1)
				return "still-stale", time.Time{}, "rt", nil
			},
			writeback: func(at string, _ time.Time, rt string) error { return nil },
		}

		err := rest.Call(t.Context(), sink,
			"https://api.example.com", "GET", "/api/test", nil, nil, nil, tok, httpCallOptions{})
		require.Error(t, err)
		var loginErr backenderr.LoginRequiredError
		assert.ErrorAs(t, err, &loginErr)
		assert.Equal(t, int32(2), calls.Load(), "retry once, then surface")
		assert.Equal(t, int32(1), refreshCalls.Load(), "refresh fires only once even if retry still 401s")
	})

	t.Run("concurrent 401s dedupe to a single refresh-grant exchange", func(t *testing.T) {
		t.Parallel()

		// A Pulumi operation can fan out parallel API calls, so an expired access token may
		// come back as N concurrent 401s. The wrapper must dedupe into a single refresh
		// exchange rather than N.
		const N = 5

		rest := newRESTClient(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") == "token fresh" {
				return &http.Response{
					StatusCode: 200, Status: "200 OK",
					Header: http.Header{}, Body: io.NopCloser(bytes.NewReader([]byte(`{}`))),
				}, nil
			}
			return &http.Response{
				StatusCode: 401, Status: "401 Unauthorized",
				Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errBody401)),
			}, nil
		})

		var refreshCalls atomic.Int32
		firstRefresh := make(chan struct{})
		releaseRefresh := make(chan struct{})

		tok := &refreshableAPIAccessToken{
			accessToken:  "stale",
			refreshToken: "rt",
			refresh: func(_ context.Context, _ string) (string, time.Time, string, error) {
				if refreshCalls.Add(1) == 1 {
					close(firstRefresh)
					<-releaseRefresh
				}
				return "fresh", time.Time{}, "rt", nil
			},
			writeback: func(_ string, _ time.Time, _ string) error { return nil },
		}

		results := make(chan error, N)
		for range N {
			go func() {
				results <- rest.Call(t.Context(), sink,
					"https://api.example.com", "GET", "/api/test", nil, nil, nil, tok, httpCallOptions{})
			}()
		}

		// Once the first goroutine is in refresh, give the rest a moment to queue on the wrapper's
		// mutex — no Go primitive surfaces "goroutine blocked on Mutex."
		<-firstRefresh
		time.Sleep(100 * time.Millisecond)
		close(releaseRefresh)

		for range N {
			require.NoError(t, <-results,
				"all concurrent callers should authenticate after the deduped refresh")
		}
		assert.Equal(t, int32(1), refreshCalls.Load(),
			"concurrent 401s must dedupe to a single /api/oauth/token call")
	})
}

//nolint:paralleltest //  subtests mutate the global otel tracer provider.
func TestDoCreatesPerAttemptSpans(t *testing.T) {
	t.Run("single successful request", func(t *testing.T) {
		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		prev := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		t.Cleanup(func() {
			otel.SetTracerProvider(prev)
		})

		client := &defaultHTTPClient{
			&http.Client{
				Transport: &errorTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Status:     "200 OK",
							Body:       io.NopCloser(bytes.NewReader(nil)),
						}, nil
					},
				},
			},
		}

		req, err := http.NewRequest("GET", "https://api.example.com/test", nil)
		require.NoError(t, err)

		resp, err := client.Do(req, retryAllMethods)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		tp.ForceFlush(t.Context())
		spans := recorder.Ended()

		// Should have an "HTTP attempt" span nested under an "HTTP request" span.
		attemptSpans := filterSpansByName(spans, "HTTP attempt")
		require.Len(t, attemptSpans, 1)
		assertSpanAttribute(t, attemptSpans[0], "http.attempt", int64(1))
		assertSpanAttribute(t, attemptSpans[0], "http.status_code", int64(200))
	})

	t.Run("retries on 500", func(t *testing.T) {
		recorder := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
		prev := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		t.Cleanup(func() {
			otel.SetTracerProvider(prev)
		})

		var callCount atomic.Int32
		client := &defaultHTTPClient{
			&http.Client{
				Transport: &errorTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						n := callCount.Add(1)
						if n <= 2 {
							return &http.Response{
								StatusCode: 500,
								Status:     "500 Internal Server Error",
								Body:       io.NopCloser(bytes.NewReader(nil)),
							}, nil
						}
						return &http.Response{
							StatusCode: 200,
							Status:     "200 OK",
							Body:       io.NopCloser(bytes.NewReader(nil)),
						}, nil
					},
				},
			},
		}

		req, err := http.NewRequest("GET", "https://api.example.com/test", nil)
		require.NoError(t, err)

		resp, err := client.Do(req, retryAllMethods)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		tp.ForceFlush(t.Context())
		spans := recorder.Ended()

		attemptSpans := filterSpansByName(spans, "HTTP attempt")
		require.Len(t, attemptSpans, 3, "expected 2 failed + 1 successful attempt")

		// Verify attempt numbers are sequential.
		for i, span := range attemptSpans {
			assertSpanAttribute(t, span, "http.attempt", int64(i+1))
		}
		// First two should have 500 status, last should have 200.
		assertSpanAttribute(t, attemptSpans[0], "http.status_code", int64(500))
		assertSpanAttribute(t, attemptSpans[1], "http.status_code", int64(500))
		assertSpanAttribute(t, attemptSpans[2], "http.status_code", int64(200))
	})
}

func filterSpansByName(spans []sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	var result []sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == name {
			result = append(result, s)
		}
	}
	return result
}

func assertSpanAttribute(t *testing.T, span sdktrace.ReadOnlySpan, key string, want int64) {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			require.Equal(t, want, attr.Value.AsInt64(), "attribute %s", key)
			return
		}
	}
	require.Failf(t, "attribute not found", "attribute %q not found on span %q", key, span.Name())
}
