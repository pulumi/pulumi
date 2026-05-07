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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

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

//nolint:paralleltest // mutates the package-level userAgentExtras state observed by UserAgent()
func TestUserAgentExtras(t *testing.T) {
	// Snapshot whatever the current state is and restore it on exit so this
	// test cannot leak into others.
	t.Cleanup(func() {
		SetUserAgentCommand("")
		SetUserAgentAIAgent("")
	})

	SetUserAgentCommand("")
	SetUserAgentAIAgent("")
	base := UserAgent()
	assert.Regexp(t, `^pulumi-cli/1 \([^()]*\)$`, base,
		"baseline UserAgent should be a single comment with no cmd=/agent= fields")
	assert.NotContains(t, base, "cmd=")
	assert.NotContains(t, base, "agent=")

	SetUserAgentCommand("stack ls")
	withCmd := UserAgent()
	assert.Contains(t, withCmd, "; cmd=stack-ls)",
		"command names with spaces are normalized to dashes")
	assert.NotContains(t, withCmd, "agent=")

	SetUserAgentAIAgent("claude")
	withBoth := UserAgent()
	assert.Contains(t, withBoth, "; cmd=stack-ls;")
	assert.Contains(t, withBoth, "; agent=claude)")

	SetUserAgentCommand("")
	withAgentOnly := UserAgent()
	assert.NotContains(t, withAgentOnly, "cmd=")
	assert.Contains(t, withAgentOnly, "; agent=claude)")

	// Sanitization strips characters that would break the User-Agent comment
	// grammar.
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
