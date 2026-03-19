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
			context.Background(), opentracing.NoopTracer{}.StartSpan("test"),
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
				context.Background(), opentracing.NoopTracer{}.StartSpan("test"),
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
			context.Background(), opentracing.NoopTracer{}.StartSpan("test"),
			diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			httpClient, "https://api.pulumi.com", "GET", "/api/test", nil,
			apiAccessToken(""), httpCallOptions{},
		)

		assert.True(t, errors.Is(err, backenderr.LoginRequiredError{}))
	})
}
