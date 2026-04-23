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
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestRawCall_GzipRequestCompression pins that POST bodies are gzip
// compressed on the wire and Content-Encoding: gzip is set when gzipBody is
// true.
func TestRawCall_GzipRequestCompression(t *testing.T) {
	t.Parallel()
	var gotEncoding string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEncoding = r.Header.Get("Content-Encoding")
		gr, err := gzip.NewReader(r.Body)
		require.NoError(t, err)
		gotBody, err = io.ReadAll(gr)
		require.NoError(t, err)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "", false, nil)
	resp, err := c.RawCall(t.Context(), http.MethodPost, "/x", nil,
		[]byte(`{"name":"acme"}`),
		http.Header{"Content-Type": []string{"application/json"}}, true)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "gzip", gotEncoding)
	assert.JSONEq(t, `{"name":"acme"}`, string(gotBody))
}

// TestRawCall_GzipResponseDecompression pins that gzip-encoded response
// bodies are transparently decoded and that Content-Encoding and
// Content-Length headers are removed (both apply to the compressed bytes
// and would mislead any consumer that inspects them after decompression).
func TestRawCall_GzipResponseDecompression(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		encoding string
	}{
		{"gzip", "gzip"},
		{"x-gzip", "x-gzip"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := `{"ok":true,"msg":"hello"}`
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			_, err := gw.Write([]byte(payload))
			require.NoError(t, err)
			require.NoError(t, gw.Close())

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", tc.encoding)
				w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
				_, _ = w.Write(buf.Bytes())
			}))
			t.Cleanup(srv.Close)

			c := NewClient(srv.URL, "", false, nil)
			resp, err := c.RawCall(t.Context(), http.MethodGet, "/x", nil, nil, nil, false)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.JSONEq(t, payload, string(body))
			assert.Empty(t, resp.Header.Get("Content-Encoding"),
				"Content-Encoding must be stripped so consumers don't double-decode")
			assert.Empty(t, resp.Header.Get("Content-Length"),
				"Content-Length applied to the compressed bytes and is meaningless after decompression")
		})
	}
}

// TestRawCall_WarningHeaderPassthrough pins that X-Pulumi-Warning response
// headers are surfaced through the diag sink supplied to the Client.
func TestRawCall_WarningHeaderPassthrough(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Pulumi-Warning", "deprecated field in use")
		w.Header().Add("X-Pulumi-Warning", "rate limit approaching")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	var stderr bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &stderr, diag.FormatOptions{Color: colors.Never})
	c := NewClient(srv.URL, "", false, sink)
	resp, err := c.RawCall(t.Context(), http.MethodGet, "/x", nil, nil, nil, false)
	require.NoError(t, err)
	resp.Body.Close()

	out := stderr.String()
	assert.Contains(t, out, "deprecated field in use")
	assert.Contains(t, out, "rate limit approaching")
}

// TestRawCall_DoesNotClassify4xx pins that RawCall returns the raw response
// for 4xx/5xx instead of synthesizing a typed error like Call does. Callers
// inspect the status code themselves.
func TestRawCall_DoesNotClassify4xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "tok", false, nil)
	resp, err := c.RawCall(t.Context(), http.MethodGet, "/bad", nil, nil, nil, false)
	require.NoError(t, err, "RawCall must not turn 4xx into an error")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"message":"nope"}`, string(body))
}

// trackingReadCloser records whether Close has been called, so the test can
// verify gzipReadCloser forwards Close to the underlying body.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error { t.closed = true; return nil }

// TestGzipReadCloser_ClosesUnderlyingBody pins that closing the wrapper
// closes both the gzip reader and the underlying response body.
func TestGzipReadCloser_ClosesUnderlyingBody(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte(`{"ok":true}`))
	require.NoError(t, gw.Close())

	body := &trackingReadCloser{Reader: bytes.NewReader(buf.Bytes())}
	gr, err := gzip.NewReader(body)
	require.NoError(t, err)

	wrapper := &gzipReadCloser{gzip: gr, body: body}
	_, err = io.ReadAll(wrapper)
	require.NoError(t, err)
	require.NoError(t, wrapper.Close())
	assert.True(t, body.closed, "underlying body must be closed when the wrapper is closed")
}

// TestRawCall_AppendsPulumiAcceptVersion pins that the API version is
// appended to caller-supplied Accept so the server still routes through
// versioned content negotiation. Go's http.Header preserves multiple Accept
// values as a slice, so check via Values, not Get.
func TestRawCall_AppendsPulumiAcceptVersion(t *testing.T) {
	t.Parallel()
	var gotAccept []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Values("Accept")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "", false, nil)
	resp, err := c.RawCall(t.Context(), http.MethodGet, "/x", nil, nil,
		http.Header{"Accept": []string{"application/json"}}, false)
	require.NoError(t, err)
	resp.Body.Close()

	joined := strings.Join(gotAccept, ", ")
	assert.Contains(t, joined, "application/json")
	assert.Contains(t, joined, "application/vnd.pulumi+8",
		"the transport must always advertise the Pulumi API version")
}
